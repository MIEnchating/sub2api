# Vue I18n srcdoc backport

`intlify-shared-9.14.5-srcdoc.patch` mitigates GHSA-83rm-6c8r-g728 in the
application's Vue I18n runtime. Escaping an iframe's `srcdoc` value is insufficient:
the browser decodes it into a nested HTML document. The patch neutralizes the
attribute name, which HTML parsers do not entity-decode. It handles whitespace,
case variations, and the slash attribute separator, while retaining ordinary
translation markup and interpolation.

The patch covers every JavaScript implementation in `@intlify/shared` (ESM,
CommonJS, browser, and production variants); forwarding entry points reuse those
implementations. Vite and Vitest explicitly select
`vue-i18n/dist/vue-i18n.runtime.esm-bundler.js`, which imports core-base and this
patched shared package. Vue I18n/core-base standalone browser and global bundles
embed their own sanitizer and are **not covered**; do not switch to them.

The lockfile records the patch hash. Both source Dockerfiles copy patches before
the frozen dependency install. `pnpm build`, including the Release frontend job,
runs `translationHtmlSecurity.spec.ts` against the same runtime entry point. Its
six attack cases fail without the patch and verify the browser-parsed attribute,
not just the returned string. Ordinary rich translations are also checked.

The three exact advisory exceptions expire on 2026-11-28 (they intentionally do
not use the non-expiring `origin: upstream` policy). Remove the patch and those
exceptions when Vue I18n and its message compiler can be migrated together to a
supported release >=11.4.14. During this repair the registry was unreachable and
only 9.14.5 was cached, so a new major version and its integrity metadata could
not be verified.

The other failed advisory, GHSA-qr28-fpjm-wq7f, was removed from the dependency
graph by deleting the unused `@lobehub/icons` dependency. ModelIcon and
ProviderIcon use checked-in SVG paths; neither imports that React package.
