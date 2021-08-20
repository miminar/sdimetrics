{ pkgs ? import <nixpkgs> { } }:

with pkgs;
let
  drv = callPackage ./default.nix { };

  goPackagePath = "github.com/miminar/sdimetrics";
in
drv.overrideAttrs (attrs: {
  src = null;
  nativeBuildInputs = [ govers go ] ++ attrs.nativeBuildInputs;
  shellHook = ''
    echo 'Entering ${attrs.pname}'
    set -v
    export GOPATH="$(pwd)/.go"
    export GOCACHE=""
    export GO111MODULE='on'
    go mod init ${goPackagePath}
    set +v
  '';
})
