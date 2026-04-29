# Changelog

## 1.0.0 (2026-04-29)


### Features

* **api:** add stats endpoint and tighten platform-admin authz ([3a50f84](https://github.com/danielpadua/oad/commit/3a50f847a0892d019f0987bb92feab671ffece70))
* **backlog:** mark design system components as completed in Phase 7.4 ([256f271](https://github.com/danielpadua/oad/commit/256f27122938c1c6c06e2d828cfb4f62cc47cf43))
* Enhance mergeJSON function to support nested JSON merging ([257c905](https://github.com/danielpadua/oad/commit/257c905fdfb60e8dd044cfeb5f68ce576bdb8d9a))
* **feat-phase7.4:** implement role-based access control with system scope management and enhance UI components ([459ec55](https://github.com/danielpadua/oad/commit/459ec5500025935d24378c6fb6e6c6b3ad10831f))
* implement phase 2 schema registry (entity types, systems, overlay schemas) ([2d94d63](https://github.com/danielpadua/oad/commit/2d94d639a55fba2983e3fd71438884dae4a59128))
* implement phase 3 entity and relation management ([cf7bf8b](https://github.com/danielpadua/oad/commit/cf7bf8bd48afa1f5cb80272648d7911308db9d27))
* implement phase 4 overlay system (property_overlay CRUD) ([35d720b](https://github.com/danielpadua/oad/commit/35d720b563462c90dd645e024598e179538e4391))
* phase 1 cross-cutting middleware, local dev tooling, and CLAUDE.md update ([79ea964](https://github.com/danielpadua/oad/commit/79ea964fa1a28672757df255b741f1b8a2ffbc09))
* phase 5 — retrieval API (FR-RET-001..004) ([a310ee6](https://github.com/danielpadua/oad/commit/a310ee6b019e80d0ddf390b13e701282206521b2))
* **phase-6:** implement webhook subscription management ([7ff4dc6](https://github.com/danielpadua/oad/commit/7ff4dc67d9652ffeb6232d520c009ca45f22b569))
* **phase-7.1:** Add Docker support for frontend management UI with Vite and Nginx ([89110b4](https://github.com/danielpadua/oad/commit/89110b4010831e20cf96b87963f5715b30bac63f))
* **phase-7.2:** implement OIDC authentication with Keycloak and support multi-provider JWT validation ([8c293fe](https://github.com/danielpadua/oad/commit/8c293fe6261075c81db556ae53ff17a418ebb88a))
* **phase-7:** add System and Webhook management pages with CRUD functionality ([9b96f64](https://github.com/danielpadua/oad/commit/9b96f64e598eef3ef85506cab6563482228eb616))
* **web:** add frontend model (phase 7.1) ([af1a755](https://github.com/danielpadua/oad/commit/af1a75531412db184f3f7a7ee9fb6c17552b1bc4))
* **web:** implement design system and feedback primitives (phase 7.4) ([aea66c0](https://github.com/danielpadua/oad/commit/aea66c065b9a00ca0ca4a1ac60d2bff116491d29))
* **web:** implement i18n with EN/PT-BR support and wire all pages ([637a161](https://github.com/danielpadua/oad/commit/637a161a2cfe820421c710eb7855b95b3c57b8ff))


### Bug Fixes

* **lint:** align struct field comments to satisfy gofumpt tabwriter rules ([7ef8dc4](https://github.com/danielpadua/oad/commit/7ef8dc4ba058169772018dea79813ca671aa2d93))
* **lint:** correct gofumpt formatting in params.go and router.go ([dad5ef6](https://github.com/danielpadua/oad/commit/dad5ef6b2cc0df9a511a3840b23b8689b840e9cf))
* **lint:** replace naked return with explicit return values in parsePagination ([fceded2](https://github.com/danielpadua/oad/commit/fceded2faa733b3dceece1167397c5e2d1336207))
* resolve golangci-lint errcheck and gosec G115 issues ([7c47721](https://github.com/danielpadua/oad/commit/7c47721bceb679484bff008ba04af9f1ac186cf1))


### Code Refactoring

* update golangci-lint configuration to use exclude-rules for web directories and add test configuration file ([6879f1b](https://github.com/danielpadua/oad/commit/6879f1babc07b865ff47884cbf8e9ad9b729c609))
