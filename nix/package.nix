{ lib
, buildGoModule
, makeWrapper
, fzf
, xclip
, wl-clipboard
}:

let
  pname = "kpass";
  version = "0.3.2";
in
buildGoModule {
  inherit pname version;

  src = lib.cleanSource ./..;

  vendorHash = "sha256-KPNAB8/DrmFh6R+APSQCHNdPrp+E7WLA7cX1yWhY+EI=";

  subPackages = [ "cmd/kpass" ];

  ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];

  doCheck = true;

  nativeBuildInputs = [ makeWrapper ];

  postInstall = ''
    wrapProgram $out/bin/kpass \
      --prefix PATH : ${lib.makeBinPath [ fzf xclip wl-clipboard ]}

    mkdir -p $out/share/bash-completion/completions
    $out/bin/kpass completion bash > $out/share/bash-completion/completions/kpass

    mkdir -p $out/share/zsh/site-functions
    $out/bin/kpass completion zsh > $out/share/zsh/site-functions/_kpass

    mkdir -p $out/share/fish/vendor_completions.d
    $out/bin/kpass completion fish > $out/share/fish/vendor_completions.d/kpass.fish
  '';

  meta = {
    description = "Another CLI for KeePass";
    homepage = "https://github.com/irasikhin/kpass";
    license = lib.licenses.mit;
    mainProgram = "kpass";
    platforms = lib.platforms.unix;
  };
}
