{
  description = "Sync Jira releases with GitHub releases";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    go-overlay.url = "github:purpleclay/go-overlay";
    nix-oci.url = "github:dauliac/nix-oci";
    nopher.url = "github:anthr76/nopher";
  };

  outputs = inputs@{ flake-parts, nixpkgs, go-overlay, nopher, ... }:
    let
      mkPkgs = system: import nixpkgs {
        inherit system;
        overlays = [
          go-overlay.overlays.default
          nopher.overlays.default
        ];
      };
      mkDevShell = system:
        let
          gopkgs = mkPkgs system;
        in
        gopkgs.mkShell {
          buildInputs = [
            (gopkgs.go-bin.fromGoMod ./go.mod)
            gopkgs.just
          ];
        };
      mkPackage = system:
        let
          gopkgs = mkPkgs system;
        in
        gopkgs.buildGoModule {
          pname = "jira-release-sync";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          subPackages = [ "./cmd/jira-release-sync" ];
          env.CGO_ENABLED = "0";
        };
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        inputs.nix-oci.flakeModule
      ];
      systems = [ "x86_64-linux" "aarch64-linux" ];
      oci.enabled = true;
      perSystem = { config, self', inputs', pkgs, system, ... }:
        let
          gopkgs = mkPkgs system;
          go = gopkgs.go-bin.fromGoMod ./go.mod;
          jira-release-sync = mkPackage system;
        in
        {
          devShells.default = gopkgs.mkShell {
            buildInputs = [
              go
              pkgs.just
            ];
          };

          packages.default = jira-release-sync;

          oci.containers.jira-release-sync = {
            package = jira-release-sync;
            entrypoint = [ "/bin/jira-release-sync" ];
            push = false;
            multiArch = {
              enabled = true;
              tempTagPrefix = "tmp";
            };
            tags = [
              "latest"
            ];
          };

          oci.debug = {
            enabled = true;
            entrypoint.enabled = true;
            packages = with pkgs; [
              coreutils
              bash
              curl
            ];
          };
        };
      flake = {
        devShells.aarch64-darwin.default = mkDevShell "aarch64-darwin";
        devShells.x86_64-darwin.default = mkDevShell "x86_64-darwin";
        packages.aarch64-darwin.default = mkPackage "aarch64-darwin";
        packages.x86_64-darwin.default = mkPackage "x86_64-darwin";
      };
    };
}
