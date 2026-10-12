{
  lib,
  schema,
  metadata,
}:

let
  optionType =
    rule:
    if rule.type == "bool" then
      schema.types.bool
    else if rule.type == "choice" then
      schema.types.enum (builtins.attrNames rule.choices)
    else if rule.type == "int" then
      schema.types.enum rule.allowed_ints
    else
      schema.types.enum rule.allowed_strings;

  description = rule: ''
    ${rule.description}
    Enforced policy `${rule.policy}`; this is not an editable profile default.
    Supported versions: ${lib.strings.concatStringsSep "; " rule.supported_on}.
    ${rule.account_scope or ""}
    Reference: ${rule.source}
  '';

  nested = lib.attrsets.foldlAttrs (
    options: name: rule:
    lib.attrsets.recursiveUpdate options (
      lib.attrsets.setAttrByPath (lib.strings.splitString "." name) (
        schema.optional (optionType rule) (description rule)
      )
    )
  ) { } metadata.entries;

  settings = lib.attrsets.mapAttrs (
    _: value:
    if lib.options.isOption value then
      value
    else
      schema.optionalSubmodule value "Product-specific managed policies."
  ) nested;
in
{
  inherit settings;
  hasPolicyIntent =
    configured:
    configured != null
    && (
      lib.lists.any (
        name: lib.attrsets.attrByPath (lib.strings.splitString "." name) null configured != null
      ) (builtins.attrNames metadata.entries)
      || ((configured.managed_policies or null) != null && configured.managed_policies != { })
    );
}
