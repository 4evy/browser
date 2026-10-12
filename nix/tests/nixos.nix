{
  tweezlePackage,
  nixosModule,
  pkgs,
}:

let
  tweezleExecutable = pkgs.lib.meta.getExe tweezlePackage;

  fakeBrowser = pkgs.writeShellScript "fake-browser" (builtins.readFile ./nixos/fake-browser.sh);

  fakeDesktop = pkgs.writeText "fake-browser.desktop" (
    builtins.readFile ./nixos/fake-browser.desktop
  );

  fakeIcon = pkgs.writeText "product_logo_256.png" "fake browser icon";

  fakeBrowserBundle = pkgs.runCommand "fake-browser-bundle" {
    inherit fakeBrowser fakeDesktop fakeIcon;
  } (builtins.readFile ./nixos/bundle.sh);

  launcherFlags = pkgs.writeText "vm-browser-flags.conf" (builtins.readFile ./nixos/launcher.conf);

  expectedArguments = pkgs.writeText "vm-browser-expected-arguments" (
    builtins.readFile ./nixos/expected-arguments.txt
  );

  vmChecks =
    pkgs.runCommand "tweezle-vm-checks"
      {
        nativeBuildInputs = [ pkgs.go_1_27 ];
        env.GO111MODULE = "off";
      }
      ''
        export GOCACHE="$TMPDIR/go-cache"
        go build -o "$out" ${./nixos/nixos.go}
      '';
in
pkgs.testers.nixosTest {
  name = "tweezle-module";

  nodes.machine =
    {
      modulesPath,
      pkgs,
      ...
    }:
    {
      imports = [
        (modulesPath + "/profiles/minimal.nix")
        nixosModule
      ];

      programs.tweezle = {
        enable = true;
        package = tweezlePackage;
        settings.chromium = {
          name = "VM Browser";
          executable_name = "vm-browser";
          alias_name = "vm-browser-alias";
          flags_file = "tweezle/vm-browser-flags.conf";
          flags = [ "--from-config" ];

          linux = {
            desktop_id = "vm-browser";
            portal_app_id = "org.chromium.Chromium";
            wrapper_flags = [ "--from-linux" ];
            launcher_name = "fake-browser";
            desktop_name = "fake-browser.desktop";
            desktop_exec = "fake-browser";
            icon_name = "vm-browser.png";
            icon_source = "product_logo_256.png";
          };

          paths.linux.profile_dir = "\${config_home}/vm-browser/Default";

          preferences.values = [
            {
              path = "vm.checked";
              value = true;
            }
          ];
        };

        configurations.secondary.chromium = {
          name = "Secondary VM Browser";
          executable_name = "secondary-vm-browser";
        };
      };

      users.users.alice = {
        isNormalUser = true;
        description = "Browser VM test user";
      };

      environment.systemPackages = [ pkgs.jq ];
      documentation.enable = false;
      system.stateVersion = "25.11";
      virtualisation.memorySize = 512;
    };

  # The upstream driver boots the VM; all guest checks run in Go
  testScript = ''
    machine.start()
    machine.wait_for_unit("multi-user.target")
    machine.succeed(${
      builtins.toJSON (
        pkgs.lib.strings.escapeShellArgs [
          vmChecks
          launcherFlags
          tweezleExecutable
          fakeBrowserBundle
          fakeIcon
          expectedArguments
          ./nixos/launcher.jq
        ]
      )
    })
  '';
}
