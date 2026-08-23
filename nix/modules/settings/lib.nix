{
  lib,
  tomlFormat,
}:

let
  inherit (lib) mkOption types;
in
rec {
  inherit types;

  freeformSubmodule =
    options:
    types.submodule {
      freeformType = tomlFormat.type;
      inherit options;
    };

  required = type: description: mkOption { inherit type description; };

  optional =
    type: description:
    required (types.nullOr type) description
    // {
      default = null;
    };

  optionalBool = optional types.bool;
  optionalInt = optional types.int;
  optionalNonEmptyString = optional types.nonEmptyStr;
  optionalString = optional types.str;
  optionalStringList = optional (types.listOf types.str);
  optionalSubmodule = options: optional (freeformSubmodule options);
  optionalUnsigned = optional types.ints.unsigned;

  extensionId = types.strMatching "[a-p]{32}";
  pathString = types.coercedTo types.path toString types.str;
}
