# Совместимость профиля и миграция

## Backend profile

v1.1 и v2.0 используют одну schema `1` и один Windows path:

```text
%APPDATA%\IrisOnlineDatabase\UserData\profile.json
```

Сохраняются server/theme/view, favorites, history и recently viewed. Каталожные поиски и фильтры намеренно остаются transient и сбрасываются при новом запуске. Неизвестные валидные top-level JSON fields сохраняются при чтении/перезаписи.

Профиль ограничен по размеру, записывается через temporary file + atomic replace с backup `%APPDATA%\IrisOnlineDatabase\UserData\Backups\profile.json.bak`. Если primary повреждён, загружается backup; иначе создаётся безопасный default.

## Переход browser origin → WebView2

Текущая v1.1 при первом запуске уже переносит legacy `localStorage` в backend profile. v2 читает именно этот backend profile, поэтому смена browser origin не теряет данные. Pending frontend snapshot дополнительно хранится в стабильном `%LOCALAPPDATA%\IrisOnlineDatabase\WebView2`; при следующем запуске он имеет приоритет до успешной backend-записи.

Для прямого перехода с версии до появления backend profile сначала запустите последнюю v1.1 один раз и дождитесь главного экрана, затем закройте её штатно и запускайте v2. Это ограничение безопасности браузерных origins: v2 не читает и не разбирает чужой Edge/Chrome profile напрямую.

Перед обновлением можно скопировать папку `%APPDATA%\IrisOnlineDatabase\UserData`; installer/release EXE её не удаляет.
