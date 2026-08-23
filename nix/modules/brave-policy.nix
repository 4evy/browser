{ lib }:

let
  policyFeatures = [
    "wallet"
    "rewards"
    "ipfs"
    "ai_chat"
    "local_ai"
    "vpn"
    "news"
    "talk"
    "playlist"
    "web_discovery"
    "p3a"
    "stats"
    "email_aliases"
    "speedreader"
    "wayback_machine"
    "psst"
    "tor"
  ];
in
{
  hasPolicyIntent =
    settings:
    let
      configuredBrave = settings.browser.brave or null;
      brave = if configuredBrave == null then { } else configuredBrave;
      configuredFeatures = brave.features or null;
      features = if configuredFeatures == null then { } else configuredFeatures;
      configuredPolicies = brave.managed_policies or null;
    in
    (brave.origin or null) == true
    || (brave.disable_web3 or null) == true
    || (brave.disable_annoyances or null) == true
    || (configuredPolicies != null && configuredPolicies != { })
    || lib.any (name: (features.${name} or null) != null) policyFeatures;
}
