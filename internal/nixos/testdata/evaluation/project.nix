{
  config,
  lib,
  pkgs,
  pinned,
  unstableSource,
  ...
}:
let
  expected = builtins.fromJSON (builtins.readFile ./expected.json);
in
{
  lmx.capabilities.languageSupport.languages.python.parsers = [ "python" ];
  # A port that the configuration declares too, and a range: the configuration of lmx lists each once.
  networking.firewall.allowedTCPPorts = [ 8080 ];
  networking.firewall.allowedTCPPortRanges = [
    {
      from = 9000;
      to = 9002;
    }
  ];
  # A project module explains itself in the guest like a catalog module.
  limanix.help.project = {
    summary = "Tools of the project.";
    commands = [ "hello" ];
    tips = [
      {
        label = "Greet";
        text = "hello";
      }
    ];
  };
  # NixOS accepts contextual strings and string-like store objects as packages.
  environment.systemPackages = [
    "${pkgs.hello}"
    { __toString = _: toString pkgs.hello; }
    (
      pkgs.hello
      // {
        meta = pkgs.hello.meta // {
          priority = 7;
        };
      }
    )
  ];
  assertions = [
    {
      assertion =
        config.limanix.user.name == "eval-user"
        && config.users.users.eval-user.uid == 501
        && config.limanix.user.home == "/home/eval-user";
      message = "The client must populate the public guest identity.";
    }
    {
      assertion = builtins.elem pkgs.ghostty.terminfo config.environment.systemPackages;
      message = "Terminal compatibility must not require a catalog module.";
    }
    {
      assertion = !config.programs.neovim.enable;
      message = "Declaring a language must not activate an editor.";
    }
    {
      assertion =
        !config.nix.gc.automatic
        && config.nix.settings.auto-optimise-store
        && !config.documentation.doc.enable;
      message = "lmxd, not a timer, collects the guest store, and documentation outputs stay out.";
    }
    {
      assertion =
        config.nix.settings.min-free == 1073741824
        && config.nix.settings.max-free == 2147483648
        && !(config.systemd.services ? limanix-store-guard)
        && !(config.systemd.timers ? limanix-store-guard)
        && lib.hasInfix "SystemMaxUse=512M" config.services.journald.extraConfig;
      message = "The platform must keep 10-20% of the default guest disk free during builds, lmxd checks between them, and the journal stays bounded.";
    }
    {
      assertion = builtins.any (
        package: (package.pname or "") == "lmx"
      ) config.environment.systemPackages;
      message = "Every guest must reach lmx, the Mac clipboard and sessions without a catalog module.";
    }
    {
      assertion =
        let
          # Tools are store paths, and fromJSON rejects their string context.
          text = builtins.unsafeDiscardStringContext config.environment.etc."lmx/config.json".text;
          settings = builtins.fromJSON text;
          tools = builtins.removeAttrs settings.tools [ "sudo" ];
        in
        builtins.attrNames settings == [
          "disk"
          "dotfiles"
          "generation"
          "health"
          "modules"
          "network"
          "schema"
          "session"
          "theme"
          "tools"
          "user"
          "vm"
        ]
        && settings.dotfiles == "/mnt/dotfiles"
        && settings.modules == expected.modules
        &&
          builtins.attrNames settings.tools == [
            "bash"
            "grep"
            "ionice"
            "ip"
            "journalctl"
            "nice"
            "nix_env"
            "nix_store"
            "nixos_rebuild"
            "sudo"
            "systemctl"
            "systemd_run"
          ]
        && settings.generation == "0123456789ab"
        &&
          settings.user == {
            name = "eval-user";
            home = "/home/eval-user";
            uid = 501;
            gid = 100;
          }
        && builtins.elem "users" config.users.users.eval-user.extraGroups
        &&
          settings.network.ports == {
            tcp = [
              22
              8080
              9000
              9001
              9002
            ];
            udp = [ ];
          }
        &&
          settings.health.units == [
            "sshd.service"
            "lima-guestagent.service"
            "lmx.socket"
          ]
        && settings.theme.name == "fixture-theme"
        && settings.theme.palette.background == "#112233"
        && builtins.length (builtins.attrNames settings.theme.palette) == 18
        && settings.session.providers == config.limanix.session.providers
        && builtins.all (path: lib.hasPrefix "/nix/store/" path) (builtins.attrValues tools)
        && settings.tools.sudo == "/run/wrappers/bin/sudo";
      message = "lmx must receive the platform configuration of its generation.";
    }
    {
      assertion =
        let
          help = builtins.fromJSON config.environment.etc."lmx/help.json".text;
        in
        help.schema == 1
        &&
          builtins.attrNames help.topics == [
            "fixture"
            "project"
          ]
        &&
          help.topics.project == {
            title = "project";
            summary = "Tools of the project.";
            commands = [ "hello" ];
            tips = [
              {
                label = "Greet";
                text = "hello";
              }
            ];
            guide = null;
          };
      message = "lmx must receive the help cards of the evaluated modules.";
    }
    {
      assertion =
        builtins.elem "sockets.target" config.systemd.sockets.lmx.wantedBy
        && builtins.elem "multi-user.target" config.systemd.services.lmx.wantedBy
        && config.systemd.services.lmx.serviceConfig.Type == "notify"
        && !config.systemd.services.lmx.restartIfChanged
        && builtins.any (check: lib.hasPrefix "lmx-config-check" check.name) config.system.checks;
      message = "lmxd must run from boot, answer on its socket and accept the configuration of its generation.";
    }
    {
      assertion =
        config.programs.nix-ld.enable
        && builtins.any (library: (library.pname or "") == "libxext") config.programs.nix-ld.libraries
        &&
          config.users.users.eval-user.openssh.authorizedKeys.keys
          == [ "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEvaluationKeyOfTheDevelopmentAccount lima" ]
        && builtins.elem "d /etc/ssh/authorized_keys.d 0755 root root -" config.systemd.tmpfiles.rules
        && builtins.any (
          package: (package.pname or "") == "gnutar" && package.meta.priority < 0
        ) config.environment.systemPackages;
      message = "Editors on the Mac must reach the development account over SSH and run their servers in every guest.";
    }
    {
      assertion =
        !config.nix.channel.enable
        && builtins.elem "r /nix/var/nix/profiles/per-user/root/channels-*-link" config.systemd.tmpfiles.rules;
      message = "The base image's channel must not remain a garbage-collector root.";
    }
    {
      assertion =
        builtins.all (package: lib.count (entry: entry == package) config.environment.systemPackages == 1) [
          pkgs.tree
          pkgs.python3Packages.ipython
          pkgs.terraform
        ]
        && builtins.all (name: builtins.elem name config.nixpkgs.config.allowUnfreePackages) [
          "tree"
          "ipython"
          "terraform"
          "cowsay"
        ]
        && builtins.any (check: lib.hasPrefix "limanix-packages-check" check.name) config.system.checks;
      message = "Listed packages must be installed once, allowed by name and checked for command conflicts.";
    }
    {
      assertion =
        config.lmx.pins.${unstableSource.rev} == unstableSource.sha256
        && builtins.elem pinned.${unstableSource.rev}.cowsay config.environment.systemPackages
        && !(builtins.elem pkgs.cowsay config.environment.systemPackages);
      message = "An unstable: package must come from the catalog's nixos-unstable revision.";
    }
    {
      assertion =
        lib.hasInfix "lmx dotfiles link --quiet" config.programs.bash.interactiveShellInit
        && lib.hasInfix "lmx dotfiles link --quiet" config.programs.zsh.shellInit
        && lib.hasInfix "[[ -o interactive ]]" config.programs.zsh.shellInit
        && !(lib.hasInfix "lmx dotfiles" config.environment.interactiveShellInit)
        && config.users.users.eval-user.shell == pkgs.bashInteractive;
      message = "Interactive shells must link the dotfiles before they read the account's startup files.";
    }
  ];
}
