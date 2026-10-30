{
  config,
  lib,
  pkgs,
  ...
}:
let
  runtime = builtins.fromJSON (builtins.readFile ./runtime.json);
  pin = builtins.fromJSON (builtins.readFile ./lmx.json);
  inherit (pkgs.stdenv.hostPlatform) system;
  archive = pin.systems.${system} or (throw "lmx ${pin.version} has no release for ${system}");
  inherit (config.lmx.capabilities) theme;

  # A development build of the client bundles the archive beside this file instead of its address.
  src =
    if archive ? file then
      let
        local = ./. + "/${archive.file}";
      in
      assert lib.assertMsg (builtins.hashFile "sha256" local == archive.sha256)
        "lmx ${pin.version}: ${archive.file} does not match its sha256";
      local
    else
      pkgs.fetchurl { inherit (archive) url sha256; };

  # Static lmx and lmxd from the pinned release for the guest's system.
  release = pkgs.stdenvNoCC.mkDerivation {
    pname = "lmx-release";
    inherit (pin) version;
    inherit src;
    dontConfigure = true;
    dontBuild = true;
    dontFixup = true;
    installPhase = ''
      runHook preInstall
      install -D -m 0755 -t "$out/bin" lmx lmxd
      install -D -m 0644 -t "$out/share/licenses/lmx" LICENSE
      runHook postInstall
    '';
  };

  # Commands for people. The clipboard and session commands are links to lmx, which dispatches
  # on the name it runs under. lmxd stays out of PATH.
  commands =
    pkgs.runCommand "lmx-${pin.version}"
      {
        pname = "lmx";
        inherit (pin) version;
      }
      ''
        mkdir -p "$out/bin"
        for name in lmx pbcopy pbpaste limanix-session; do
          ln -s ${release}/bin/lmx "$out/bin/$name"
        done
      '';

  # Ports the firewall opens, as NixOS evaluated them, including the ports that modules open.
  ports =
    protocol:
    let
      inherit (config.networking) firewall;
      ranges = lib.concatMap (range: lib.range range.from range.to) firewall."allowed${protocol}PortRanges";
      sorted = lib.sort lib.lessThan (firewall."allowed${protocol}Ports" ++ ranges);
      # One pass keeps a wide range cheap: in sorted order a repeat equals its predecessor.
      repeat = index: port: index > 0 && builtins.elemAt sorted (index - 1) == port;
    in
    lib.filter (port: port != null) (
      lib.imap0 (index: port: if repeat index port then null else port) sorted
    );

  # The account's links, after the shell-specific `guard`.
  linkDotfiles = guard: ''
    if ${guard}[ "$(${pkgs.coreutils}/bin/id -u)" = "${toString runtime.user.uid}" ]; then
      ${commands}/bin/lmx dotfiles link --quiet || :
    fi
  '';

  # lmx 0.1 rejects unknown fields, so the key appears only with dotfiles.
  dotfiles = lib.optionalAttrs (runtime.shell.dotfiles != null) { inherit (runtime.shell) dotfiles; };

  settings = dotfiles // {
    schema = 1;
    vm = {
      inherit (runtime) name arch;
      system = "NixOS ${config.system.nixos.release}";
    };
    inherit (runtime) generation;
    user = {
      inherit (config.limanix.user) name home;
      inherit (runtime.user) uid;
      gid = config.ids.gids.users;
    };
    modules = runtime.selectedModules;
    disk = {
      collect_percent = runtime.disk.collectPercent;
      minimum_percent = runtime.disk.minimumPercent;
    };
    health.units = [
      (if config.services.openssh.startWhenNeeded then "sshd.socket" else "sshd.service")
      "lima-guestagent.service"
      "lmx.socket"
    ];
    network.ports = {
      tcp = ports "TCP";
      udp = ports "UDP";
    };
    # lmx 0.1 rejects unknown fields, so the terminal colors appear only with the system theme.
    theme = {
      inherit (theme) name palette;
    }
    // lib.optionalAttrs (theme.name == "system") { inherit (theme) ansi; };
    session = {
      command =
        if config.limanix.session.command == null then null else toString config.limanix.session.command;
      inherit (config.limanix.session) providers;
    };
    # Each tool is named by its path: lmxd runs its tools with a cleared environment.
    tools = {
      ip = lib.getExe' pkgs.iproute2 "ip";
      systemctl = lib.getExe' config.systemd.package "systemctl";
      nix_store = lib.getExe' config.nix.package "nix-store";
      nice = lib.getExe' pkgs.coreutils "nice";
      ionice = lib.getExe' pkgs.util-linux "ionice";
      grep = lib.getExe' pkgs.gnugrep "grep";
      nixos_rebuild = lib.getExe' config.system.build.nixos-rebuild "nixos-rebuild";
      nix_env = lib.getExe' config.nix.package "nix-env";
      sudo = "${config.security.wrapperDir}/sudo";
      bash = lib.getExe' pkgs.bashInteractive "bash";
      systemd_run = lib.getExe' config.systemd.package "systemd-run";
      journalctl = lib.getExe' config.systemd.package "journalctl";
    };
  };

  # Help cards that the evaluated modules declare, for lmx help TOPIC.
  help = {
    schema = 1;
    topics = lib.mapAttrs (_: card: {
      inherit (card)
        title
        summary
        commands
        guide
        ;
      tips = map (tip: { inherit (tip) label text; }) card.tips;
    }) config.limanix.help;
  };

  # This generation's lmx, lmxd, configuration and help cards, for an lmxd that runs before the
  # system is built. lmx reads the help cards beside the configuration.
  generation = pkgs.runCommand "lmx-generation-${runtime.generation}" { } ''
    mkdir -p "$out/bin" "$out/etc/lmx"
    ln -s ${release}/bin/lmx ${release}/bin/lmxd "$out/bin/"
    ln -s ${config.environment.etc."lmx/config.json".source} "$out/etc/lmx/config.json"
    ln -s ${config.environment.etc."lmx/help.json".source} "$out/etc/lmx/help.json"
  '';

  # The release rejects unknown and missing fields. A configuration or help cards it cannot read
  # fail the system build here, before a restart, instead of every lmx command after it.
  configCheck = pkgs.runCommand "lmx-config-check" { } ''
    if ! page=$(LMX_CONFIG=${generation}/etc/lmx/config.json ${release}/bin/lmx help); then
      printf '%s\n' "$page" | grep 'cannot be read' >&2
      exit 1
    fi
    touch "$out"
  '';
in
{
  # The system profile and /run/booted-system carry this file too: lmx reads their generation
  # markers from it.
  environment.etc."lmx/config.json".text = builtins.toJSON settings;
  environment.etc."lmx/help.json".text = builtins.toJSON help;
  environment.systemPackages = [ commands ];
  system.build.lmx = generation;
  system.checks = [ configCheck ];

  # The group in user.gid owns the environment files that lmxd installs.
  users.users.${config.limanix.user.name}.extraGroups = [ "users" ];

  # The reference units of the lmx repository, with store paths.
  systemd.sockets.lmx = {
    description = "Socket of the LimaNix guest owner";
    wantedBy = [ "sockets.target" ];
    socketConfig = {
      ListenStream = "/run/lmx/lmx.sock";
      # Every user may connect; lmxd decides what each caller may do from its peer credentials.
      SocketMode = "0666";
    };
  };
  systemd.services.lmx = {
    description = "LimaNix guest owner";
    requires = [ "lmx.socket" ];
    after = [ "lmx.socket" ];
    # Started at boot, not only by its socket: the store guard and the generation observer must run.
    wantedBy = [ "multi-user.target" ];
    # A switch never stops lmxd in the middle of a task. The new lmxd starts with the next boot,
    # like the rest of a generation.
    restartIfChanged = false;
    serviceConfig = {
      Type = "notify";
      ExecStart = "${release}/bin/lmxd";
      # lmxd pings at half this interval; a frozen daemon is aborted and restarted.
      WatchdogSec = "30s";
      Restart = "on-failure";
      # Task processes live in this unit's control group and stop with it.
      KillMode = "control-group";
      # A watchdog abort would otherwise store a core of every process in the unit.
      LimitCORE = 0;
    };
  };

  # Every interactive shell of the account links the dotfiles before it reads the account's own
  # startup files: Zsh from /etc/zshenv, ahead of ~/.zshenv, and Bash from /etc/bashrc, which
  # /etc/profile reads ahead of ~/.bash_profile.
  programs.zsh.shellInit = lib.mkIf (runtime.shell.dotfiles != null) (
    linkDotfiles "[[ -o interactive ]] && "
  );
  programs.bash.interactiveShellInit = lib.mkIf (runtime.shell.dotfiles != null) (linkDotfiles "");

  environment.interactiveShellInit = ''
    if [ -t 1 ] && [ -z "''${LIMANIX_WELCOME_SHOWN:-}" ] \
      && [ "$(${pkgs.coreutils}/bin/id -u)" = "${toString runtime.user.uid}" ]; then
      export LIMANIX_WELCOME_SHOWN=1
      ${commands}/bin/lmx welcome || :
    fi
  '';
}
