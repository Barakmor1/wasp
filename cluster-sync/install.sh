#!/usr/bin/env bash

set -e

function install_wasp {
  _kubectl apply -f "./_out/manifests/release/wasp.yaml"
}

function delete_wasp {
  _kubectl delete -f "./_out/manifests/release/wasp.yaml"
}