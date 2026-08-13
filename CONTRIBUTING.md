# Участие в разработке

Работайте только в одном клоне репозитория. Не копируйте source в соседний `github-repo`, `release-files` или временный publish tree.

Перед pull request:

1. Прочитайте `docs/BUILD.md` и используйте Go из `.go-version`.
2. Обновляйте код, тесты, CI и документацию в одном изменении.
3. Не изменяйте игровые assets механически. Для осознанного изменения обновите соответствующий builder, audit, checksum и объяснение.
4. Не добавляйте secrets, EXE, coverage, caches, virtualenv, `.syso` или Playwright downloads в source.
5. Запустите `python -B tools/repository_audit.py`, Go/Python tests, Ruff format/check и связанные data audits.

Python tools запускаются как `python -B tools/name.py`, поэтому в них нет shebang и executable bit. Shell release scripts, напротив, имеют shebang и executable mode.

Новая production dependency требует обоснования, проверки лицензии и security review. Vanilla frontend не использует npm framework; Wails — единственная новая desktop runtime dependency v2.
