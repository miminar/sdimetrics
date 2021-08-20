{ buildGoModule
, nix-gitignore
}:

buildGoModule {
  pname = "sdimetrics";
  version = "0.0.1";
  src = nix-gitignore.gitignoreSource [ ] ./.;
  #goPackagePath = "github.com/miminar/sdimetrics";
  #modSha256 = "0000000000000000000000000000000000000000000000000000";
  #vendorSha256 = "0000000000000000000000000000000000000000000000000000";
  vendorSha256 = null;
}
