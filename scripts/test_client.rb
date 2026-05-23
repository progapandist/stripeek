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

Stripe.api_key  = load_key
Stripe.api_base = PROXY_BASE

puts "→ talking to Stripe via #{PROXY_BASE}"
puts "→ account #{ACCOUNT_ID}"
puts

# 1. Create a test customer.
customer = Stripe::Customer.create(
  email: "stripeek+#{Time.now.to_i}@example.com",
  description: "stripeek smoke test",
  metadata: { source: "stripeek-test-client" }
)
puts "✓ created customer #{customer.id}"

# 2. List the last 3 customers.
list = Stripe::Customer.list(limit: 3)
puts "✓ listed #{list.data.size} customers"

# 3. Retrieve the one we just made.
fetched = Stripe::Customer.retrieve(customer.id)
puts "✓ retrieved #{fetched.id}  (email: #{fetched.email})"

# 4. Create a PaymentIntent (test card flow, no confirmation).
intent = Stripe::PaymentIntent.create(
  amount: 1234,
  currency: "usd",
  customer: customer.id,
  description: "stripeek test charge"
)
puts "✓ created payment_intent #{intent.id}  (status: #{intent.status})"

# 5. Deliberately trigger a 4xx so the TUI shows a red status.
begin
  Stripe::Customer.retrieve("cus_does_not_exist_xxxxxxxxxxxxx")
rescue Stripe::InvalidRequestError => e
  puts "✓ provoked expected 404 — #{e.message[0, 60]}…"
end

puts
puts "done. check the TUI."
