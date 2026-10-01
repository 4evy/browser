{ lib, ... }:

{
  _class = "nixos";

  meta.maintainers = [ lib.maintainers._4evy ];

  imports = [ ./system.nix ] ++ (import ../providers { inherit lib; }).nixosModules;
}
