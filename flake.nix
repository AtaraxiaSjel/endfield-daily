{
  inputs.nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  outputs =
    { nixpkgs, ... }:
    let
      forAllSystems =
        function:
        nixpkgs.lib.genAttrs
          [
            "x86_64-linux"
            "aarch64-linux"
          ]
          (
            system:
            function (
              import nixpkgs {
                inherit system;
                config.allowUnfree = true;
              }
            )
          );
    in
    {
      packages = forAllSystems (pkgs: rec {
        endfield-daily = pkgs.callPackage ./default.nix { };
        default = endfield-daily;
      });
    };
}
