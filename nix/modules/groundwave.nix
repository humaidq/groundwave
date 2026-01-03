# Copyright 2025 Humaid Alqasimi
# SPDX-License-Identifier: Apache-2.0
{
  config,
  lib,
  pkgs,
  ...
}:

with lib;

let
  cfg = config.services.groundwave;
in
{
  options.services.groundwave = {
    enable = mkEnableOption "Groundwave personal CRM with amateur radio logging";

    port = mkOption {
      type = types.port;
      default = 8080;
      description = "Port for the web interface to listen on";
    };

    envFile = mkOption {
      type = types.path;
      description = ''
        Path to environment file containing secrets.
        Should include: CARDDAV_URL, CARDDAV_USERNAME, CARDDAV_PASSWORD, and ZK settings.
      '';
    };
  };

  config = mkIf cfg.enable {
    # PostgreSQL setup
    services.postgresql = {
      enable = true;

      ensureDatabases = [ "groundwave" ];

      ensureUsers = [
        {
          name = "groundwave";
          ensureDBOwnership = true;
        }
      ];
    };

    # System user and group
    users.users.groundwave = {
      isSystemUser = true;
      group = "groundwave";
      description = "Groundwave service user";
    };

    users.groups.groundwave = { };

    # Systemd service
    systemd.services.groundwave = {
      description = "Groundwave Personal CRM";
      wantedBy = [ "multi-user.target" ];
      after = [
        "network.target"
        "postgresql.service"
      ];
      requires = [ "postgresql.service" ];

      serviceConfig = {
        Type = "simple";
        User = "groundwave";
        Group = "groundwave";
        Restart = "on-failure";
        RestartSec = "5s";

        # State directory handling
        StateDirectory = "groundwave";
        WorkingDirectory = "/var/lib/groundwave";

        # Load secrets from envFile
        EnvironmentFile = cfg.envFile;

        # Security hardening
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ReadWritePaths = [ "/var/lib/groundwave" ];

        # Base environment variables
        Environment = [
          "DATABASE_URL=postgres:///groundwave"
        ];
      };

      script = ''
        exec ${pkgs.groundwave}/bin/groundwave start --port ${toString cfg.port}
      '';
    };
  };
}
