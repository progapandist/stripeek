# Fires a handful of Stripe API calls through the local stripeek proxy.
#
# Key lookup order:
#   1. macOS Keychain entry  service="stripeek" account="sk_test"   (recommended)
#   2. STRIPE_API_KEY env var
#
# To store the key in Keychain (one time):
#   security add-generic-password -s stripeek -a sk_test -w 'sk_test_...'
#
# To read it back manually:
#   security find-generic-password -s stripeek -a sk_test -w
#
# To remove it:
#   security delete-generic-password -s stripeek -a sk_test

require "bundler/inline"

gemfile do
  source "https://rubygems.org"
  gem "stripe", "~> 13.0"
end

require "stripe"
require "open3"

PROXY_BASE = ENV.fetch("STRIPEEK_BASE", "http://localhost:4111")
ACCOUNT_ID = "acct_1AGUCWB3ZHLBhbGB".freeze

RUN_TAG = "#{Time.now.strftime("%m%d-%H%M%S")}-#{rand(1000)}".freeze

def load_key
  out, status = Open3.capture2("security", "find-generic-password",
                               "-s", "stripeek", "-a", "sk_test", "-w")
  return out.strip if status.success? && !out.strip.empty?

  key = ENV.fetch("STRIPE_API_KEY", nil)
  return key if key && !key.empty?

  abort <<~MSG
    No Stripe key found.

    Store it in macOS Keychain (recommended):
      security add-generic-password -s stripeek -a sk_test -w 'sk_test_...'

    Or export it for this shell only:
      export STRIPE_API_KEY=sk_test_...
  MSG
end

def step(label)
  result = yield
  puts "✓ #{label}"
  result
end

Stripe.api_key  = load_key
Stripe.api_base = PROXY_BASE

puts "→ talking to Stripe via #{PROXY_BASE}"
puts "→ account #{ACCOUNT_ID}"
puts "→ run tag #{RUN_TAG}"
puts

# 1. Customer
customer = step("created customer") do
  Stripe::Customer.create(
    email: "stripeek+#{RUN_TAG}@example.com",
    name: "Stripeek Test #{RUN_TAG}",
    description: "stripeek smoke test #{RUN_TAG}",
    metadata: { source: "stripeek-test-client", run: RUN_TAG }
  )
end

# 2. List
step("listed customers") { Stripe::Customer.list(limit: 3) }

# 3. Retrieve
step("retrieved #{customer.id}") { Stripe::Customer.retrieve(customer.id) }

# 4. Attach a test payment method so the customer can hold subscriptions.
pm = step("attached test payment method") do
  pm = Stripe::PaymentMethod.create(
    type: "card",
    card: { token: "tok_visa" }
  )
  Stripe::PaymentMethod.attach(pm.id, customer: customer.id)
  Stripe::Customer.update(customer.id, invoice_settings: { default_payment_method: pm.id })
  pm
end

# 5. Product + two prices (monthly starter → monthly pro).
product = step("created product") do
  Stripe::Product.create(
    name: "Stripeek Pro #{RUN_TAG}",
    metadata: { source: "stripeek-test-client", run: RUN_TAG }
  )
end

price_starter = step("created starter price ($9/mo)") do
  Stripe::Price.create(
    product: product.id,
    currency: "usd",
    unit_amount: 900,
    recurring: { interval: "month" },
    nickname: "Starter #{RUN_TAG}"
  )
end

price_pro = step("created pro price ($29/mo)") do
  Stripe::Price.create(
    product: product.id,
    currency: "usd",
    unit_amount: 2900,
    recurring: { interval: "month" },
    nickname: "Pro #{RUN_TAG}"
  )
end

# 6. Subscription schedule: 14-day trial on Starter, then auto-upgrade to Pro.
#    This is where Stripe payloads get deeply nested — phases, add_invoice_items,
#    billing_thresholds, trial_settings, default_settings, proration_behavior, etc.
now = Time.now.to_i
schedule = step("created subscription schedule (trial → pro)") do
  Stripe::SubscriptionSchedule.create(
    customer: customer.id,
    start_date: now,
    end_behavior: "release",
    metadata: { source: "stripeek-test-client", experiment: "trial-to-pro" },
    default_settings: {
      collection_method: "charge_automatically",
      default_payment_method: pm.id,
      invoice_settings: { days_until_due: nil },
      billing_thresholds: nil,
      automatic_tax: { enabled: false }
    },
    phases: [
      {
        items: [{ price: price_starter.id, quantity: 1 }],
        trial_end: now + (14 * 24 * 60 * 60),
        end_date: now + (14 * 24 * 60 * 60),
        proration_behavior: "none",
        billing_thresholds: nil,
        collection_method: "charge_automatically",
        metadata: { phase: "trial" }
      },
      {
        items: [
          { price: price_starter.id, quantity: 1 },
          { price: price_pro.id, quantity: 1 }
        ],
        proration_behavior: "create_prorations",
        collection_method: "charge_automatically",
        billing_thresholds: { amount_gte: 10_000, reset_billing_cycle_anchor: true },
        add_invoice_items: [
          {
            price_data: {
              currency: "usd",
              product: product.id,
              unit_amount: 500
            },
            quantity: 1
          }
        ],
        metadata: { phase: "pro" }
      }
    ]
  )
end

puts "  schedule id: #{schedule.id}  status: #{schedule.status}"
puts "  phases: #{schedule.phases.size}  " \
     "(#{schedule.phases.map { |p| "#{p.items.size} item(s)" }.join(" → ")})"

# 7. Retrieve the schedule — plain retrieve (deep payload already).
step("retrieved schedule") { Stripe::SubscriptionSchedule.retrieve(schedule.id) }

# 8. Retrieve the prices and product separately — each has a rich nested payload.
step("retrieved starter price") { Stripe::Price.retrieve(price_starter.id) }
step("retrieved pro price") { Stripe::Price.retrieve(price_pro.id) }
step("retrieved product") { Stripe::Product.retrieve(product.id) }

# 9. PaymentIntent for a one-off charge.
step("created payment_intent") do
  Stripe::PaymentIntent.create(
    amount: 1234,
    currency: "usd",
    customer: customer.id,
    description: "stripeek test charge",
    payment_method: pm.id,
    confirm: true,
    off_session: true
  )
end

# 10. Deliberate 4xx — red status in the TUI.
begin
  Stripe::Customer.retrieve("cus_does_not_exist_xxxxxxxxxxxxx")
rescue Stripe::InvalidRequestError => e
  puts "✓ provoked expected 404 — #{e.message[0, 60]}…"
end

puts
puts "done. check the TUI."
