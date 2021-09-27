#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

root="$(readlink -f "$(dirname "${BASH_SOURCE[0]}")/..")"

set -x

tmpd="$(mktemp -d)"
function cleanup() {
    rm -rf "$tmpd"
}
trap cleanup EXIT

pushd "$tmpd"
    csplit "$root/k8s/ds-node-exporter.yaml" '%^---$%1' '/^---$/' '{*}'
    pullSpec=
    dsFile=
    for f in *; do
        jq . <(yaml2json "$f") > "$f.json"
        if [[ -n "${pullSpec:-}" ]]; then
            continue
        fi
        pullSpec="$(jq -r 'if .kind == "DaemonSet" then
            .spec.template.spec.containers[].image | sub("[@].*"; "")
        else
            ""
        end' "$f.json" | head -n 1)"
        if [[ -n "${pullSpec:-}" ]]; then
            dsFile="$f.json"
        fi
    done

    digest="$(skopeo inspect "docker://$pullSpec" | jq -r .Digest)"
    if [[ -z "${digest:-}" ]]; then
        printf 'Could not determine digest for image "%s"!\n' >&2 "$pullSpec"
        exit 1
    fi

    jq --arg pullSpec "${pullSpec%%:*}@${digest}" \
        '.spec.template.spec.containers |= [.[] | .image |= $pullSpec]' \
        "$dsFile" | sponge "$dsFile"

    for f in *.json; do
        printf -- '---\n' >>result.yaml
        json2yaml "$f" >>result.yaml
    done
popd

git stash
    git checkout digest
        git fetch
        git rebase origin/master
        cp "$tmpd/result.yaml" "$root/k8s/ds-node-exporter.yaml"
        git commit -vasm 'updated digest for the node exporter'
        git push -f
    git checkout -
git stash pop
