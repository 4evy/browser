{ lib }:

let
  metadata = lib.trivial.importJSON ../metadata.json;
  policyFeatures = builtins.attrNames (
    lib.attrsets.filterAttrs (_: feature: (feature.policies or [ ]) != [ ]) metadata.features
  );
  presetFeatures =
    name:
    metadata.presets.${name}.features or [ ]
    ++ lib.lists.concatMap presetFeatures (metadata.presets.${name}.extends or [ ]);
in
rec {
  hasPolicyIntent =
    settings:
    let
      brave = lib.trivial.defaultTo { } (settings.chromium.brave or null);
      features = lib.trivial.defaultTo { } (brave.features or null);
      configuredPolicies = brave.managed_policies or null;
      behavior = lib.trivial.defaultTo { } (brave.behavior or null);
      standalone = (brave.edition or null) == "origin";
      needsPolicy =
        name:
        lib.lists.elem name policyFeatures
        && !(standalone && lib.lists.elem name metadata.editions.origin.features);
    in
    lib.lists.any (
      name: (brave.${name} or null) == true && lib.lists.any needsPolicy (presetFeatures name)
    ) (builtins.attrNames metadata.presets)
    ||
      ((brave.disable_annoyances or null) == true || (brave.origin or null) == true)
      && (behavior.show_default_browser_prompt or null) != true
    || (configuredPolicies != null && configuredPolicies != { })
    || (behavior.show_default_browser_prompt or null) == false
    || lib.lists.any (name: (features.${name} or null) != null && needsPolicy name) policyFeatures;
  isConfigured =
    cfg:
    lib.lists.any hasPolicyIntent (
      lib.lists.optional (cfg.settings != null) cfg.settings ++ builtins.attrValues cfg.configurations
    );
}
