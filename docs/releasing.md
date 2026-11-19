# Cutting a release

teakit is distributed as a Go module. Pushing a root Git tag makes that version available to Go, even before the GitHub Release exists. Tag a commit that has already passed CI on `main`.

1. Choose a `v0.MINOR.PATCH` tag. Use a patch for compatible fixes, and a minor for new APIs, breaking changes to the API or key behavior, or a higher Go minimum.
2. Include `docs/releases/<tag>.md` in the release commit. Describe what a user will notice. For an upgrade that needs action, show the old call or behavior, its replacement, and any new dependency minimum. CI uses this file as the release body and appends GitHub's generated change list.
3. After merging, copy the full commit SHA from the successful **Build and test source bundle** run. For example:

   ```sh
   git fetch origin main
   tag=v0.1.0
   commit='paste-the-tested-commit-SHA-here'
   go run ./internal/cmd/check-release "$tag" "$commit" &&
     git tag -a "$tag" "$commit" -m "$tag" &&
     git push origin "refs/tags/$tag"
   ```

4. The tag run packages and tests that Git tree, then publishes the same archive and `SHA256SUMS` to [Releases](https://github.com/undervoke/teakit/releases). Confirm the release notes and assets there, and check that the public Go proxy can resolve the tag:

   ```sh
   curl -fsSL "https://proxy.golang.org/github.com/undervoke/teakit/@v/$tag.info" |
     jq -es --arg tag "$tag" '.[0].Version == $tag'
   ```

For a transient CI or upload failure, rerun the failed jobs for the same tag. If an upload left an unpublished draft, remove that draft while keeping the tag, then retry. For a source correction, publish a new version: Go proxies and consumers may already have cached the original tag and its checksum.
