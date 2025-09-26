# EPG Filter

Быстрый фильтр XMLTV файлов на Go. Заменяет медленные bash-скрипты с xmlstarlet.

## Особенности

- Обрабатывает файлы 200+ МБ за секунды
- Не требует зависимостей (один бинарный файл)
- Простой в использовании
- Тестировался и подходит для XMLTV EPG от https://iptvx.one

## Установка

```bash
# Скачайте бинарный файл
wget https://github.com/xak666/epg-filter/releases/download/v1.0/epg_filter
chmod +x epg_filter

## Использование
# Показать все каналы
./epg_filter epg.xml

# Фильтровать каналы
./epg_filter epg.xml "channel1,channel2" output.xml

## Компиляция
go build -o epg_filter epg_filter.go
