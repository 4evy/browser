{ lib, ... }:

{
  _class = "nixos";

  imports = [ ./system.nix ] ++ (import ../providers { inherit lib; }).nixosModules;
}
