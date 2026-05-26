{
  description = "kpass — another CLI for KeePass";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      overlay = final: prev: {
        kpass = final.callPackage ./nix/package.nix { };
      };
    in
    {
      overlays.default = overlay;
    }
    // flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        };
      in
      {
        packages = {
          default = pkgs.kpass;
          kpass = pkgs.kpass;
        };

        apps.default = {
          type = "app";
          program = "${pkgs.kpass}/bin/kpass";
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gotools
            pkgs.delve
            pkgs.fzf
            pkgs.xclip
            pkgs.wl-clipboard
          ];
        };

        checks.default = pkgs.kpass;
      });
}
