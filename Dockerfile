# Используем лёгкий alpine образ для запуска
FROM alpine:latest

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем бинарь в контейнер
COPY wedding-backend .

# Делаем бинарь исполняемым (на всякий случай)
RUN chmod +x wedding-backend

# Открываем порт, если нужно (например, 8080)
EXPOSE 8080

# Запускаем приложение
CMD ["./wedding-backend"]

