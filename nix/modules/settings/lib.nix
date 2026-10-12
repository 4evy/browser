{
  lib,
  tomlFormat,
}:

let
  inherit (lib) types;
in
rec {
  inherit types;

  freeformSubmodule = options: freeformSubmoduleWith [ { inherit options; } ];

  freeformSubmoduleWith =
    modules:
    types.submoduleWith {
      shorthandOnlyDefinesConfig = true;
      modules = [ { freeformType = tomlFormat.type; } ] ++ modules;
    };

  # Retain the source location when adapting an option set into a module
  optionsModule =
    path: args:
    lib.modules.setDefaultModuleLocation (toString path) {
      options = import path args;
    };

  required = type: description: lib.options.mkOption { inherit type description; };

  optional =
    type: description:
    lib.options.mkOption {
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
  optionalSubmoduleWith = modules: optional (freeformSubmoduleWith modules);
  optionalSubmodule = options: optionalSubmoduleWith [ { inherit options; } ];
  optionalUnsigned = optional types.ints.unsigned;

  pathString = types.coercedTo types.path toString types.str;
}
