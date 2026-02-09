#!/usr/bin/env bash

set -euo pipefail

HEAP_BEFORE="heap_before.pprof"
HEAP_AFTER="heap_after.pprof"

start_time=$(date +%s)

echo "Запускаем сбор heap профиля перед нагрузкой..."
curl -s -o "${HEAP_BEFORE}" "http://localhost:8080/debug/pprof/heap" &
HEAP_BEFORE_PID=$!

# Дожидаемся завершения сбора профиля
wait "$HEAP_BEFORE_PID" 2>/dev/null || true

echo "Запускаем 2000 автотестов с интервалом 20 мс между стартами..."
for i in {1..1000}; do
    ./shortenertest.exe -test.v -test.run=^TestIteration15$ -binary-path=./shortener.exe -database-dsn=$DATABASE_DSN > /dev/null &
    sleep 0.02

	./shortenertest.exe -test.v -test.run=^TestIteration12$ -binary-path=./shortener.exe -database-dsn=$DATABASE_DSN > /dev/null &
	sleep 0.02
done

echo "Ждём завершения тестов..."
wait

echo "Запускаем сбор heap профиля после нагрузки..."
curl -s -o "${HEAP_AFTER}" "http://localhost:8080/debug/pprof/heap" &
HEAP_AFTER_PID=$!

# Дожидаемся завершения сбора профиля
wait "$HEAP_AFTER_PID" 2>/dev/null || true

end_time=$(date +%s)
execution_time=$((end_time - start_time))

echo "Готово: heap профиль перед нагрузкой → ${HEAP_BEFORE}"
echo "Готово: heap профиль после нагрузки → ${HEAP_AFTER}"
echo "Готово: heap профиль после нагрузки и ожидания → ${HEAP_AFTER_WAIT}"
echo "Скрипт выполнялся: $execution_time секунд"


