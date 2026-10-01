{
  lib,
  tomlFormat,
}:

let
  inherit (lib) mkOption types;
in
rec {
  inherit types;

  freeformSubmodule = options: freeformSubmoduleWith [ { inherit options; } ];

  freeformSubmoduleWith =
    modules:
    types.submodule {
      freeformType = tomlFormat.type;
      imports = modules;
    };

  required = type: description: mkOption { inherit type description; };

  optional =
    type: description:
    mkOption {
      type = types.nullOr type;
      inherit description;
      default = null;
    };

  optionalBool = optional types.bool;
  optionalChoice =
    choices: name: optional (types.enum (map (option: option.name) choices.${name}.options));
  optionalInt = optional types.int;
  optionalNonEmptyString = optional types.nonEmptyStr;
  optionalString = optional types.str;
  optionalStringList = optional (types.listOf types.str);
  optionalSubmodule = options: optional (freeformSubmodule options);
  optionalUnsigned = optional types.ints.unsigned;

  pathString = types.coercedTo types.path toString types.str;
}
