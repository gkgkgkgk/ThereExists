{
  description = "ThereExists local dev environment (Go + Node + Postgres)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.go_1_22        # server toolchain (matches server/go.mod)
          pkgs.nodejs_20      # client toolchain (Vite)
          pkgs.postgresql_16  # project-local database (no Docker)
          pkgs.pgweb          # browser UI to explore the database
        ];

        shellHook = ''
          echo "∃ ThereExists dev shell"
          echo "  go   $(go version | awk '{print $3}')"
          echo "  node $(node --version)"
          echo "  psql $(psql --version | awk '{print $3}')"
          echo "  Run ./dev.sh to start everything, ./stop.sh to tear down."
        '';
      };
    };
}
