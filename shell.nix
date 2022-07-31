{ pkgs ? import <nixpkgs> {} }:
  pkgs.mkShell {
    nativeBuildInputs = [ pkgs.go_1_18 pkgs.graphviz pkgs.gv pkgs.golint ];
}
