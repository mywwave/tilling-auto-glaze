package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows/registry"
)

// WindowData представляет структуру данных окна
type WindowData struct {
	ManagedWindow struct {
		TilingSize *float64 `json:"tilingSize"`
	} `json:"managedWindow"`
}

// WebSocketMessage представляет структуру сообщения от WebSocket
type WebSocketMessage struct {
	Data WindowData `json:"data"`
}

// Глобальные переменные для управления приложением
var (
	conn       *websocket.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	isActive   bool
	mStatus    *systray.MenuItem
	mAutostart *systray.MenuItem
)

func main() {
	// Инициализируем системный трей
	systray.Run(onReady, onExit)
}

// onReady вызывается при инициализации системного трея
func onReady() {
	// Создаем иконку трея
	systray.SetIcon(getIconData())
	systray.SetTitle("TUI Yandex")
	systray.SetTooltip("TUI Yandex - Window Manager Helper")

	// Добавляем пункты меню
	mStatus = systray.AddMenuItem("Статус: Подключение...", "Статус подключения")
	mStatus.Disable()

	systray.AddSeparator()

	// Проверяем состояние автозагрузки и создаем пункт меню
	autostartEnabled := isAutostartEnabled()
	if autostartEnabled {
		mAutostart = systray.AddMenuItem("✓ Автозагрузка включена", "Отключить автозагрузку")
	} else {
		mAutostart = systray.AddMenuItem("Автозагрузка выключена", "Включить автозагрузку")
	}

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Выход", "Выход из приложения")

	// Автоматически запускаем WebSocket соединение
	go startWebSocketConnection()

	// Обработчики событий меню
	go func() {
		for {
			select {
			case <-mAutostart.ClickedCh:
				toggleAutostart()
				// Обновляем текст пункта меню
				autostartEnabled := isAutostartEnabled()
				if autostartEnabled {
					mAutostart.SetTitle("✓ Автозагрузка включена")
					mAutostart.SetTooltip("Отключить автозагрузку")
				} else {
					mAutostart.SetTitle("Автозагрузка выключена")
					mAutostart.SetTooltip("Включить автозагрузку")
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// onExit вызывается при выходе из приложения
func onExit() {
	if conn != nil {
		conn.Close()
	}
	if cancel != nil {
		cancel()
	}
}

// startWebSocketConnection запускает WebSocket соединение
func startWebSocketConnection() {
	uri := "ws://localhost:6123"

	// Создаем контекст
	ctx, cancel = context.WithCancel(context.Background())

	// Подключаемся к WebSocket серверу
	var err error
	conn, _, err = websocket.DefaultDialer.DialContext(ctx, uri, nil)
	if err != nil {
		return
	}

	isActive = true

	// Обновляем статус в меню
	if mStatus != nil {
		mStatus.SetTitle("Статус: Подключен")
	}

	// Отправляем команду подписки на события
	err = conn.WriteMessage(websocket.TextMessage, []byte("sub -e window_managed"))
	if err != nil {
		return
	}

	// Основной цикл обработки сообщений
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Читаем сообщение от сервера
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Парсим JSON сообщение
			var wsMessage WebSocketMessage
			if err := json.Unmarshal(message, &wsMessage); err != nil {
				continue
			}

			// Проверяем наличие tilingSize
			if wsMessage.Data.ManagedWindow.TilingSize == nil {
				continue
			}

			sizePercentage := *wsMessage.Data.ManagedWindow.TilingSize

			// Если размер окна меньше или равен 50%, переключаем направление tiling
			if sizePercentage <= 0.5 {
				conn.WriteMessage(websocket.TextMessage, []byte("command toggle-tiling-direction"))
			}
		}
	}
}

// stopWebSocketConnection останавливает WebSocket соединение
func stopWebSocketConnection() {
	if conn != nil {
		conn.Close()
		conn = nil
	}
	if cancel != nil {
		cancel()
	}
	isActive = false
}

// getIconData возвращает данные иконки для системного трея
func getIconData() []byte {
	// Простая иконка 16x16 в формате PNG
	// В реальном проекте здесь должна быть настоящая иконка
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x19, 0x74, 0x45, 0x58, 0x74, 0x53, 0x6F, 0x66, 0x74, 0x77, 0x61, 0x72,
		0x65, 0x00, 0x41, 0x64, 0x6F, 0x62, 0x65, 0x20, 0x49, 0x6D, 0x61, 0x67,
		0x65, 0x52, 0x65, 0x61, 0x64, 0x79, 0x71, 0xC9, 0x65, 0x3C, 0x00, 0x00,
		0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00,
		0x00, 0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

// isAutostartEnabled проверяет, включена ли автозагрузка в реестре Windows
func isAutostartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	// Получаем путь к исполняемому файлу
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	// Проверяем, есть ли запись в автозагрузке
	_, _, err = key.GetStringValue("TUIYandex")
	if err != nil {
		// Записи нет - автозагрузка выключена
		return false
	}

	// Проверяем, что путь в реестре совпадает с текущим путем
	value, _, err := key.GetStringValue("TUIYandex")
	if err != nil {
		return false
	}

	// Нормализуем пути для сравнения
	exePath = filepath.Clean(exePath)
	value = filepath.Clean(value)

	return exePath == value
}

// toggleAutostart переключает состояние автозагрузки
func toggleAutostart() {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	defer key.Close()

	// Получаем путь к исполняемому файлу
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	if isAutostartEnabled() {
		// Удаляем запись из автозагрузки
		key.DeleteValue("TUIYandex")
	} else {
		// Добавляем запись в автозагрузку
		key.SetStringValue("TUIYandex", exePath)
	}
}
