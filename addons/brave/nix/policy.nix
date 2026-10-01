{ lib }:

let
  metadata = lib.importJSON ../metadata.json;
  policyFeatures = builtins.attrNames (
    lib.filterAttrs (_: feature: (feature.policies or [ ]) != [ ]) metadata.features
  );
in
rec {
  hasPolicyIntent =
    settings:
    let
      configuredBrave = settings.browser.brave or null;
      brave = if configuredBrave == null then { } else configuredBrave;
      configuredFeatures = brave.features or null;
      features = if configuredFeatures == null then { } else configuredFeatures;
      configuredPolicies = brave.managed_policies or null;
      configuredBehavior = brave.behavior or null;
      behavior = if configuredBehavior == null then { } else configuredBehavior;
    in
    lib.any (name: (brave.${name} or null) == true) (builtins.attrNames metadata.presets)
    || (configuredPolicies != null && configuredPolicies != { })
    || (behavior.show_default_browser_prompt or null) == false
    || lib.any (name: (features.${name} or null) != null) policyFeatures;
  isConfigured =
    cfg:
    lib.any hasPolicyIntent (
      lib.optional (cfg.settings != null) cfg.settings ++ builtins.attrValues cfg.configurations
    );
}
