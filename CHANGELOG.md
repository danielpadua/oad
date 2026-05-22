# Changelog

## 1.0.0 (2026-05-22)


### Features

* **api:** add stats endpoint and tighten platform-admin authz ([3a50f84](https://github.com/danielpadua/oad/commit/3a50f847a0892d019f0987bb92feab671ffece70))
* **auth:** add GET /api/v1/me endpoint + wire resolver into server startup ([fb76eb0](https://github.com/danielpadua/oad/commit/fb76eb0823ea6995c9da6668f26dcc407420c5d8))
* **auth:** add IsSystemAdmin field and update HasRole hierarchy ([1f6a93f](https://github.com/danielpadua/oad/commit/1f6a93ff95e6f76dfd0c35506ba352e90a09cc77))
* **auth:** bootstrap admin config + startup routine ([b89fd25](https://github.com/danielpadua/oad/commit/b89fd2565ded331f3fb2a8b85a75249840c7c2d3))
* **auth:** identity cache with LRU eviction, TTL, and invalidation ([a73f465](https://github.com/danielpadua/oad/commit/a73f465d60de9e3da28259762ac16f6912cdf76c))
* **auth:** implement identity resolver with DB group/system lookup ([763ed1d](https://github.com/danielpadua/oad/commit/763ed1dca22f502143b5a7f2f16a147453ceb201))
* **auth:** JWT returns RawToken; mTLS maps OUs to Identity; remove ClaimsMapping ([da457af](https://github.com/danielpadua/oad/commit/da457af837bf6e23ffd048fb8920ff67bb8f44a7))
* **auth:** parse and validate X-OAD-System-Id header; set ActiveSystemID on identity ([cf2f9cb](https://github.com/danielpadua/oad/commit/cf2f9cb8d1ead3b7133e7fdb775689f5d47dcf3e))
* **auth:** replace claims-based Identity with DB-authoritative shape ([4187d6c](https://github.com/danielpadua/oad/commit/4187d6ceaa7c32fd1bbd6f0846bfb00d1dd16bed))
* **auth:** SCIM mutation handlers invalidate identity cache ([b91e006](https://github.com/danielpadua/oad/commit/b91e0066fb32f17eb9d37053b3ac63a542980c0e))
* **auth:** seed oad:system-admin built-in group (migration 000002) ([7dae78f](https://github.com/danielpadua/oad/commit/7dae78f291803c88b04eefcdee7b65ced9b8c6ec))
* **auth:** set IsSystemAdmin from oad:system-admin group; map SCIM oad-system-admin ([a7394d6](https://github.com/danielpadua/oad/commit/a7394d60eda75da791c1696f5054b66bc7a3814a))
* **auth:** wire resolver into auth middleware; drop pool param from Authentication ([ba32f68](https://github.com/danielpadua/oad/commit/ba32f688883d4e5b9e6d178ae671af559f325314))
* **authz:** add RequirePathSystemInScope middleware ([70170ba](https://github.com/danielpadua/oad/commit/70170badf41fb4d41622ff5ec168964a4cc8a68a))
* **backlog:** mark design system components as completed in Phase 7.4 ([256f271](https://github.com/danielpadua/oad/commit/256f27122938c1c6c06e2d828cfb4f62cc47cf43))
* **config:** reject obsolete claims_mapping key at startup (C.4) ([8799333](https://github.com/danielpadua/oad/commit/87993334be1117e8b3519f490ef524a71231088e))
* **deployments:** replace Dex+glauth with Authentik in multi-idp stack ([ed2e039](https://github.com/danielpadua/oad/commit/ed2e039fc9c10a3e02c9bdb7594ddfcaf2c21e6d))
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
* **phase-a:** adapt repositories to query System as entity ([9c8370b](https://github.com/danielpadua/oad/commit/9c8370be503c5a11f6cf6266946387ab8038f403))
* **phase-a:** rewrite initial migration for SCIM-ready schema ([abfef3d](https://github.com/danielpadua/oad/commit/abfef3d56d02b5662984dfc1a79f155d8e0d8348))
* **router:** restructure /systems routes for system-admin and editor access ([79186e4](https://github.com/danielpadua/oad/commit/79186e42721b20c973dfd9d1e4a23fb275c5152f))
* **scim:** add Groups CRUD with filter+pagination and PUT replace ([382d427](https://github.com/danielpadua/oad/commit/382d4271b261ff5a89009c5ea00cd1b6e3c77dbf))
* **scim:** add PATCH for Users and Groups with shared path parser ([24dbb34](https://github.com/danielpadua/oad/commit/24dbb348f759c1bd2d6bc5e4bb4667e5616f55ec))
* **scim:** add SCIM 2.0 router and discovery surface ([cab5dbc](https://github.com/danielpadua/oad/commit/cab5dbcccc59f52fc6e892117bf3930c3551dff3))
* **scim:** add scim-protocol-tester for SCIM 2.0 protocol-edge tests ([9456a42](https://github.com/danielpadua/oad/commit/9456a42dce35873801e9b7b6200290f9453a782a))
* **scim:** add Users LIST with filter+pagination and PUT (replace) ([827f4aa](https://github.com/danielpadua/oad/commit/827f4aab9353f3d8f7af7d5845f58a12fab3771a))
* **scim:** add Users POST/GET/DELETE with schema discovery ([e57832b](https://github.com/danielpadua/oad/commit/e57832bd617f3f31d929677215b9a918b696666d))
* **scim:** implement phase 9.b.7 CI integration and SCIM parser bugfixes ([6aefea3](https://github.com/danielpadua/oad/commit/6aefea373d77898259bc6d88d2719c474563e378))
* **system:** filter List by AllowedSystems for non-platform-admins ([577819b](https://github.com/danielpadua/oad/commit/577819b043141f88699423d2712d845731f8b977))
* **ui:** add isSystemAdmin field and hide admin-only nav items from non-admins ([1d7dc1c](https://github.com/danielpadua/oad/commit/1d7dc1ce900b91d48d7b240bf2c5c0e665dc284d))
* **ui:** consume /api/v1/me for DB-authoritative identity; attach X-OAD-System-Id header (C.6) ([9bf5cad](https://github.com/danielpadua/oad/commit/9bf5cadd26e7e0cbc91ab75a06e9101e500ada9f))
* **web:** add frontend model (phase 7.1) ([af1a755](https://github.com/danielpadua/oad/commit/af1a75531412db184f3f7a7ee9fb6c17552b1bc4))
* **web:** implement design system and feedback primitives (phase 7.4) ([aea66c0](https://github.com/danielpadua/oad/commit/aea66c065b9a00ca0ca4a1ac60d2bff116491d29))
* **web:** implement i18n with EN/PT-BR support and wire all pages ([637a161](https://github.com/danielpadua/oad/commit/637a161a2cfe820421c710eb7855b95b3c57b8ff))


### Bug Fixes

* **auth:** code quality fixes from Task 1 review ([da4cd1d](https://github.com/danielpadua/oad/commit/da4cd1d3d8621e7a26a0adb8c004acff11821b68))
* **auth:** distinguish pgx.ErrNoRows from transient errors in resolver; drop unused pool param ([4cd3cae](https://github.com/danielpadua/oad/commit/4cd3cae3e2c7355f43f38a5e39d55a3cc93895eb))
* **auth:** Provider.Name validation, cross-provider audience test, mTLS unit tests ([0a08498](https://github.com/danielpadua/oad/commit/0a084988ea94696ff6fe43671782b9b6738a381f))
* **auth:** resolve login redirect loop and role mapping for multi-idp stack ([703194c](https://github.com/danielpadua/oad/commit/703194c8cb1b4f117ce579b9de73e63cf13daa7f))
* **auth:** suppress zero-UUID in /me, log auth errors server-side, add MeHandler test ([2e9edc2](https://github.com/danielpadua/oad/commit/2e9edc254f76d913f3f625f634b973a1406bbfeb))
* **lint:** align struct field comments to satisfy gofumpt tabwriter rules ([7ef8dc4](https://github.com/danielpadua/oad/commit/7ef8dc4ba058169772018dea79813ca671aa2d93))
* **lint:** correct gofumpt formatting in params.go and router.go ([dad5ef6](https://github.com/danielpadua/oad/commit/dad5ef6b2cc0df9a511a3840b23b8689b840e9cf))
* **lint:** replace naked return with explicit return values in parsePagination ([fceded2](https://github.com/danielpadua/oad/commit/fceded2faa733b3dceece1167397c5e2d1336207))
* **multi-idp:** fix Authentik blueprint for OAuth2Provider schema in 2024.10 ([9afe893](https://github.com/danielpadua/oad/commit/9afe893bed001bc9f1216065f319f6a5d2af859b))
* **multi-idp:** resolve scim-sync-init failures on fresh stack start ([f95062b](https://github.com/danielpadua/oad/commit/f95062b3c4a6bc9c82a466e3cd1c9370e5032862))
* resolve golangci-lint errcheck and gosec G115 issues ([7c47721](https://github.com/danielpadua/oad/commit/7c47721bceb679484bff008ba04af9f1ac186cf1))
* **router:** open entity-types reads to all authenticated roles ([7da6cf2](https://github.com/danielpadua/oad/commit/7da6cf21be2751450e75c1fadc3256d5a784858c))
* **system:** distinguish nil vs empty AllowedSystems in repository.List ([0fd59e8](https://github.com/danielpadua/oad/commit/0fd59e869f775b5f67d5ab9e3b3b29cd36e547a6))


### Code Refactoring

* update golangci-lint configuration to use exclude-rules for web directories and add test configuration file ([6879f1b](https://github.com/danielpadua/oad/commit/6879f1babc07b865ff47884cbf8e9ad9b729c609))


### Documentation

* **auth:** update data-model §4.8 and CLAUDE.md for DB-authoritative auth (C.5) ([eaade08](https://github.com/danielpadua/oad/commit/eaade08f37225699c97f08188b8cba72ac0e5fc2))
* **design:** adopt authentik for scim ingest in dev stack ([bb6b64f](https://github.com/danielpadua/oad/commit/bb6b64f326d67c3984bb722e3548ddffe76232a2))
* **phase-a:** reflect schema unification in data-model and backlog ([66d4941](https://github.com/danielpadua/oad/commit/66d4941e0781a594a6fa7b6b0c942a4969a10332))
* **plans:** add system-admin role implementation plan ([2c934a9](https://github.com/danielpadua/oad/commit/2c934a924d458d9ab4b2a914ac3775335983dca0))
