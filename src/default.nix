# Copyright 2025 Humaid Alqasimi
# SPDX-License-Identifier: Apache-2.0
{ pkgs, ... }:

pkgs.buildGoModule {
  pname = "groundwave";
  version = "0.1.0";

  src = ./.;

  # The vendor hash for Go dependencies
  vendorHash = null;

  # Build from the src directory
  subPackages = [ "." ];

  meta = with pkgs.lib; {
    description = "Groundwave - Personal CRM with Amateur Radio Logging";
    homepage = "https://github.com/humaidq/groundwave";
    license = licenses.asl20;
    maintainers = [ ];
  };
}
