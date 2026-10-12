{ lib, schema }:

let
  inherit (schema) types;
  fromSchema =
    definition:
    let
      type = definition.type or "any";
      additional = definition.additionalProperties or true;
    in
    if definition ? anyOf then
      types.oneOf (map fromSchema definition.anyOf)
    else if builtins.isList type then
      types.oneOf (map (type: fromSchema (definition // { inherit type; })) type)
    else if definition ? enum then
      types.enum definition.enum
    else if type == "null" then
      types.enum [ null ]
    else if type == "boolean" then
      types.bool
    else if type == "string" then
      types.str
    else if type == "integer" then
      types.ints.between (definition.minimum or (-2147483648)) (definition.maximum or 2147483647)
    else if
      builtins.elem type [
        "number"
        "decimal"
      ]
    then
      types.oneOf [
        types.int
        types.float
      ]
    else if type == "array" then
      types.listOf (fromSchema (definition.items or { }))
    else if type == "object" then
      types.submodule {
        options = lib.attrsets.mapAttrs (
          _: child: schema.optional (fromSchema child) "Native setting field."
        ) (definition.properties or { });
        freeformType =
          if builtins.isAttrs additional then
            types.attrsOf (fromSchema additional)
          else if additional then
            types.attrsOf types.anything
          else
            null;
      }
    else
      types.anything;
  preferenceType = definition: fromSchema (definition.schema or { inherit (definition) type; });
in
{
  inherit fromSchema preferenceType;
  preferences =
    metadata:
    schema.optionalSubmodule (lib.attrsets.mapAttrs (
      name: definition:
      schema.optional (preferenceType definition) ''
        Native preference ${name}, from ${metadata.repository} at
        ${metadata.revision}, ${definition.source}. Unset values retain the
        browser's defaults. JSON objects merge; explicit arrays replace.
      ''
    ) metadata.preferences) "Source-backed fork preferences, keyed by their full native names.";
}
