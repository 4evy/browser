{
  browserPackage,
  nixosModule,
  pkgs,
}:

let
  browserExecutable = pkgs.lib.getExe browserPackage;

  fakeBrowser = pkgs.writeShellScript "fake-browser" ''
    printf '%s\n' "$@"
  '';

  fakeDesktop = pkgs.writeText "fake-browser.desktop" ''
    [Desktop Entry]
    Type=Application
    Name=Fake Browser
    Exec=fake-browser %U
  '';

  fakeIcon = pkgs.writeText "product_logo_256.png" "fake browser icon";

  fakeBrowserBundle = pkgs.runCommand "fake-browser-bundle" { } ''
    mkdir -p "$out/app"
    ln -s ${fakeBrowser} "$out/app/fake-browser"
    cp ${fakeDesktop} "$out/app/fake-browser.desktop"
    cp ${fakeIcon} "$out/app/product_logo_256.png"
  '';

  launcherFlags = pkgs.writeText "vm-browser-flags.conf" ''
    # Read by the generated launcher at runtime.
    '--from-file=two words'
  '';

  expectedArguments = pkgs.writeText "vm-browser-expected-arguments" ''
    --from-config
    --from-command
    --from-linux
    --class=vm-browser
    --from-file=two words
    --runtime
  '';
in
pkgs.testers.nixosTest {
  name = "browser-module";

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

      programs.browser = {
        enable = true;
        package = browserPackage;
        settings.browser = {
          name = "VM Browser";
          executable_name = "vm-browser";
          alias_name = "vm-browser-alias";
          flags_file = "browser/vm-browser-flags.conf";
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

        configurations.secondary.browser = {
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

  testScript = ''
    machine.start()
    machine.wait_for_unit("multi-user.target")

    with subtest("the NixOS module installs the package and generated files"):
        machine.succeed("test -x /run/current-system/sw/bin/browser")
        machine.succeed(
            "grep -F 'executable_name = \"vm-browser\"' /etc/browser/browser.toml"
        )
        machine.succeed(
            "grep -F 'executable_name = \"secondary-vm-browser\"' "
            "/etc/browser/secondary.toml"
        )

    with subtest("a normal user configures the fake browser bundle"):
        machine.succeed(
            "runuser -u alice -- mkdir -p /home/alice/.config/browser"
        )
        machine.succeed(
            "install -m644 -o alice -g users ${launcherFlags} "
            "/home/alice/.config/browser/vm-browser-flags.conf"
        )
        machine.succeed(
            "runuser -u alice -- env "
            "HOME=/home/alice "
            "XDG_CONFIG_HOME=/home/alice/.config "
            "XDG_DATA_HOME=/home/alice/.local/share "
            "${browserExecutable} configure "
            "--config=/etc/browser/browser.toml "
            "--mode=linux "
            "--root=/home/alice/.cache/browser "
            "--app-dir=${fakeBrowserBundle}/app "
            "--bin-dir=/home/alice/.local/bin "
            "--flags=--from-command"
        )

    with subtest("the launcher and its alias are native executable links"):
        machine.succeed("test -L /home/alice/.local/bin/vm-browser")
        machine.succeed(
            "test \"$(readlink /home/alice/.local/bin/vm-browser)\" = "
            "${browserExecutable}"
        )
        machine.succeed(
            "test \"$(readlink /home/alice/.local/bin/vm-browser-alias)\" = vm-browser"
        )
        machine.succeed(
            "jq -e "
            "'.command == ["
            "\"${fakeBrowserBundle}/app/fake-browser\","
            "\"--from-config\","
            "\"--from-command\","
            "\"--from-linux\","
            "\"--class=vm-browser\""
            "] and .flags_file == \"browser/vm-browser-flags.conf\"' "
            "/home/alice/.local/bin/vm-browser.browser-launcher.json"
        )

    with subtest("profile, desktop, and icon integration is applied"):
        machine.succeed(
            "jq -e '.vm.checked == true' "
            "/home/alice/.config/vm-browser/Default/Preferences"
        )
        machine.succeed(
            "grep -E '^Exec[[:space:]]*=[[:space:]]*"
            "/home/alice/.local/bin/vm-browser %U$' "
            "/home/alice/.local/share/applications/vm-browser.desktop"
        )
        machine.succeed(
            "grep -E '^StartupWMClass[[:space:]]*=[[:space:]]*vm-browser$' "
            "/home/alice/.local/share/applications/vm-browser.desktop"
        )
        machine.succeed(
            "grep -E '^NoDisplay[[:space:]]*=[[:space:]]*true$' "
            "/home/alice/.local/share/applications/org.chromium.Chromium.desktop"
        )
        machine.succeed(
            "cmp ${fakeIcon} "
            "/home/alice/.local/share/icons/hicolor/256x256/apps/vm-browser.png"
        )

    with subtest("the generated launcher executes the browser with every flag source"):
        machine.succeed(
            "runuser -u alice -- env "
            "HOME=/home/alice "
            "XDG_CONFIG_HOME=/home/alice/.config "
            "/home/alice/.local/bin/vm-browser --runtime "
            "> /tmp/vm-browser-arguments"
        )
        machine.succeed(
            "diff -u ${expectedArguments} /tmp/vm-browser-arguments"
        )
  '';
}
