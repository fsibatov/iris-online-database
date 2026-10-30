# Iris Online — primary resource drop audit, 2026-10-30 UTC

Status **PARTIAL**. This is a read-only comparison against an independently materialized user-owned resource archive; it does not authenticate the missing historical `Iris Resourses.7z` or `Iris.7z`, and it is not a release.

## Source and integrity

- Library `/Iris Resourses.rar`, RAR5, **2,544,832 bytes**, SHA-256 `2df78a9b5d4cf659462c8c08a42b2e0e6c5463209079c046576cf5e3aa3c3565`; two independent Library materializations compared byte-for-byte **PASS**. `libarchive` read all **307** regular-file entries, extracted **19,112,057** bytes with no reported data errors, and checked each extracted byte count against the RAR entry size. An independent per-entry RAR CRC32 inventory was **NOT RUN**; no claim that the RAR is identical to either missing `.7z` original. Source files were not copied into the repository.
- SHA-256 `1/kiss/item_dropn.txt`: `c880b3613ea810c84bdb06f281172a18932d9c9309c191290d5e639f7b292931`; `1/kiss/item_droplist.txt`: `39d7c1c0afaad7c14b4b41b490fbfd09cff73132644464fb6c47b1b6743ed6c0`.
- SHA-256 `1/or/item_dropn.txt`: `04e053d4ec8adb7efed67f123ad15dfa7a03ec496c45332fe06f26163794ba39`; `1/or/item_droplist.txt`: `dc647babae8cc59077e9dbf739b0e9b974a1290493c196daa42b24bce137bbd0`.
- Resource file hashes identify **242** distinct byte sequences among 307 files, **38** duplicate-hash groups and **65** redundant occurrences. These include identical world/area tables shared across server folders. They are **protected original content**; no deletion/deduplication was performed.

## Six-table exact comparison and one corrected omission

`tools/update_drop_slots.py` parsed `1/{kiss,or}/item_dropn.txt`, `item_droplist.txt`, and `item_dropw.txt` independently. Comparing their ordered data against `assets/game_data.json.gz`, omitting only the four `addAttempt*` fields that are **all zero in the primary tables** and intentionally absent in the packaged projection:

| Server | Direct monsters | Drop groups | World rules | Ordered projection after patch |
| --- | ---: | ---: | ---: | --- |
| Iris Kiss Kiss | 616 | 3,981 | 171 | **exact match** |
| The Original | 563 | 3,960 | 155 | **exact match** |

Before the patch, exactly **one** direct-drop row differed: Kiss monster ID **10042**, source line **2483**, seventh direct slot, has **two consecutive** group `6000501` entries with raw weights `1000000` and `500000` (displayed `100.0` and `50.0`); the packaged data omitted the second entry. Source line: `monster\t10042\t0\t0\t0\t0\t6000501\t1000000\t6000501\t500000`. Minimal patch restores that second ordered entry **only for Kiss**; no parser algorithm, item/monster table, other server, or chance/quantity was altered. Before: one regression **RED**, after: both regression tests **GREEN**; `tests/test_fan_disclaimer.py` pinned SHA updated to the new database hash. The RAR source independently confirms the correction. Compressed game-data SHA before: `7c3698494233696f2f5728ef17f7e13953159191f966d77b90742dbced23875e`; after: `9ad89a2fabff0f3a8375277545ee068e093f3935d09ea746508148ca1a79004b`. Semantic JSON comparison before/after confirmed exactly this one list insertion.

All **51,287** group-item rows (Kiss 25,721, Original 25,566) have a published item ID and positive quantity/chance. Primary source confirms **2** unresolved Kiss direct group references and **18** unresolved Original references (these groups are absent in the corresponding source lists), not a newly introduced parser omission. Published Original monster records still omit IDs **23007**, **23008** although direct-drop rules exist; no speculative additions. The Original and Iris Kiss Kiss remain independent.

Offline Chromium/Playwright smoke now explicitly checks two same-titled drop choices remain distinct and expanded, and item selection/back-navigation remains intact; **PASS**. The primary archive cannot establish current live-server behavior, dynamic drop modifiers, runtime penalties, or working Windows-copy equivalence.
