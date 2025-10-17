package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Структура для хранения информации о программе
type Programme struct {
	Start string
	Stop  string
	Title string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./epg_filter <input_xml> [\"channel1,channel2\"] [output_xml]")
		fmt.Println("Example: ./epg_filter epg_lite.xml \"match-tv,match-premier\" output.xml")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Printf("Error: File %s does not exist\n", inputFile)
		os.Exit(1)
	}

	var channelList string
	var outputFile string

	if len(os.Args) > 2 {
		channelList = os.Args[2]
	}
	if len(os.Args) > 3 {
		outputFile = os.Args[3]
	}

	if channelList == "" {
		displayChannels(inputFile)
		return
	}

	if outputFile == "" {
		outputFile = "filtered_" + time.Now().Format("20060102_150405") + ".xml"
	}

	err := filterXML(inputFile, outputFile, channelList)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created: %s\n", outputFile)
}

func displayChannels(inputFile string) {
	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	channels := make([]string, 0)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "<channel") && strings.Contains(line, "id=\"") {
			id := extractID(line)
			if id != "" {
				// Извлекаем первое display-name
				name := extractFirstDisplayName(line)
				if name == "" {
					name = "No name"
				}
				channels = append(channels, fmt.Sprintf("%s (%s)", id, name))
			}
			
			if len(channels) >= 100 { // Ограничим вывод 100 каналами
				break
			}
		}
	}

	fmt.Println("Available channels (first 100):")
	fmt.Println("-----------------------------")
	
	for i, channel := range channels {
		if len(channel) > 50 {
			channel = channel[:47] + "..."
		}
		fmt.Printf("%-50s", channel)
		
		if (i+1)%2 == 0 {
			fmt.Println()
		}
	}
	if len(channels)%2 != 0 {
		fmt.Println()
	}
	fmt.Printf("\nTotal channels in file: %d (showing first 100)\n", countTotalChannels(inputFile))
}

func countTotalChannels(inputFile string) int {
	file, err := os.Open(inputFile)
	if err != nil {
		return 0
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "<channel") {
			count++
		}
	}
	return count
}

func extractID(line string) string {
	if idx := strings.Index(line, "id=\""); idx != -1 {
		start := idx + 4
		rest := line[start:]
		if end := strings.Index(rest, "\""); end != -1 {
			return rest[:end]
		}
	}
	return ""
}

func extractFirstDisplayName(line string) string {
	if idx := strings.Index(line, "<display-name>"); idx != -1 {
		start := idx + 14
		rest := line[start:]
		if end := strings.Index(rest, "</display-name>"); end != -1 {
			name := rest[:end]
			// Убираем лишние пробелы
			name = strings.TrimSpace(name)
			if len(name) > 20 {
				name = name[:20] + "..."
			}
			return name
		}
	}
	return ""
}

func filterXML(inputFile, outputFile, channelList string) error {
	fmt.Printf("Filtering channels: %s\n", channelList)
	
	// Парсим каналы
	channels := strings.Split(channelList, ",")
	channelMap := make(map[string]bool)
	for _, ch := range channels {
		cleanCh := strings.TrimSpace(ch)
		if cleanCh != "" {
			channelMap[cleanCh] = true
			fmt.Printf("Looking for channel: '%s'\n", cleanCh)
		}
	}

	// Создаем выходной файл
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	// Пишем XML декларацию и заголовок
	writer.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	writer.WriteString(`<!DOCTYPE tv SYSTEM "xmltv.dtd">` + "\n")
	writer.WriteString(`<tv generator-info-name="iptvx" generator-info-url="https://megasite.ru/">` + "\n")

	// Открываем входной файл
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	
	// Счетчики
	channelsFound := 0
	programmesFound := 0
	lineCount := 0

	// Определяем диапазон дат
	now := time.Now()
	daysBack := 1   // вчера
	daysForward := 4 // сегодня + 4 дня вперед = всего 6 дней

	validDates := make(map[string]bool)

	// Добавляем дни назад
	for i := -daysBack; i <= daysForward; i++ {
    		date := now.AddDate(0, 0, i).Format("20060102")
    		validDates[date] = true
	}

	// Формируем строку для вывода
	var dateList []string
	for i := -daysBack; i <= daysForward; i++ {
    		dateList = append(dateList, now.AddDate(0, 0, i).Format("20060102"))
	}

	fmt.Printf("Filtering dates: %s\n", strings.Join(dateList, ", "))

	// Структура для хранения программ по каналам
	channelProgrammes := make(map[string][]Programme)

	// Первый проход: собираем все программы для нужных каналов за нужные даты
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		trimmed := strings.TrimSpace(line)

		// Обрабатываем channel
		if strings.HasPrefix(trimmed, "<channel") {
			id := extractID(trimmed)
			if id != "" && channelMap[id] {
				// Читаем весь блок channel
				channelBlock := readCompleteElement(scanner, trimmed, "channel")
				writer.WriteString(channelBlock + "\n")
				channelsFound++
				fmt.Printf("✓ Found channel: %s\n", id)
			}
			continue
		}

		// Обрабатываем programme
		if strings.HasPrefix(trimmed, "<programme") {
			channel := extractChannel(trimmed)
			if channel != "" && channelMap[channel] {
				start := getAttribute(trimmed, "start")
				// Проверяем дату программы
				if len(start) >= 8 {
					date := start[:8]
					if validDates[date] {
						programme := extractProgramme(scanner, trimmed)
						if programme.Start != "" && programme.Stop != "" {
							channelProgrammes[channel] = append(channelProgrammes[channel], programme)
						}
					}
				}
			}
			continue
		}
	}

	// Второй проход: сортируем программы по времени и заполняем разрывы
	for channel, programmes := range channelProgrammes {
		// Сортируем программы по времени начала
		sortedProgrammes := sortProgrammesByTime(programmes)
		
		// Заполняем временные разрывы
		filledProgrammes := fillTimeGaps(sortedProgrammes)
		
		// Записываем программы в файл
		for _, prog := range filledProgrammes {
			simpleProgramme := fmt.Sprintf("<programme start=\"%s\" stop=\"%s\" channel=\"%s\">\n<title>%s</title>\n</programme>", 
				prog.Start, prog.Stop, channel, escapeXML(prog.Title))
			writer.WriteString(simpleProgramme + "\n")
			programmesFound++
		}
		
		fmt.Printf("Channel %s: %d programmes -> %d after gap filling\n", 
			channel, len(programmes), len(filledProgrammes))
	}

	writer.WriteString("</tv>\n")
	
	fmt.Printf("\nProcessing complete:\n")
	fmt.Printf("Lines processed: %d\n", lineCount)
	fmt.Printf("Channels found: %d/%d\n", channelsFound, len(channels))
	fmt.Printf("Programmes found: %d\n", programmesFound)
	
	if channelsFound == 0 {
		fmt.Printf("ERROR: No channels found from the requested list!\n")
		fmt.Printf("Please check channel IDs and try again.\n")
	} else {
		fmt.Printf("Success! Output file: %s\n", outputFile)
	}
	
	return nil
}

func extractProgramme(scanner *bufio.Scanner, firstLine string) Programme {
	start := getAttribute(firstLine, "start")
	stop := getAttribute(firstLine, "stop")
	title := "No title"
	
	// Ищем title в следующих строках
	linesToRead := 10
	
	for i := 0; i < linesToRead; i++ {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		
		// Ищем тег title в любой форме (с атрибутами или без)
		if strings.Contains(line, "<title") && strings.Contains(line, "</title>") {
			if foundTitle := extractTitleFromLine(line); foundTitle != "" {
				title = foundTitle
				break // Нашли title, можно выходить
			}
		}
		
		if strings.Contains(line, "</programme>") {
			break
		}
	}

	// УДАЛЯЕМ все XML/HTML entities вместо преобразования
	title = removeXMLEntities(title)

	return Programme{
		Start: start,
		Stop:  stop,
		Title: title,
	}
}

func sortProgrammesByTime(programmes []Programme) []Programme {
	if len(programmes) <= 1 {
		return programmes
	}
	
	// Простая сортировка пузырьком
	for i := 0; i < len(programmes)-1; i++ {
		for j := i + 1; j < len(programmes); j++ {
			if programmes[i].Start > programmes[j].Start {
				programmes[i], programmes[j] = programmes[j], programmes[i]
			}
		}
	}
	return programmes
}

func fillTimeGaps(programmes []Programme) []Programme {
	if len(programmes) == 0 {
		return programmes
	}
	
	var result []Programme
	
	for i := 0; i < len(programmes); i++ {
		current := programmes[i]
		
		// Добавляем текущую программу
		result = append(result, current)
		
		// Проверяем разрыв до следующей программы
		if i < len(programmes)-1 {
			next := programmes[i+1]
			
			currentEnd, err1 := parseTime(current.Stop)
			nextStart, err2 := parseTime(next.Start)
			
			if err1 == nil && err2 == nil {
				// Если есть разрыв более 1 минуты
				gap := nextStart.Sub(currentEnd)
				if gap > time.Minute {
					// Создаем запись "Реклама" для заполнения разрыва
					adStart := formatTime(currentEnd)
					adStop := formatTime(nextStart)
					
					adProgramme := Programme{
						Start: adStart,
						Stop:  adStop,
						Title: "Реклама",
					}
					result = append(result, adProgramme)
				}
			}
		}
	}
	
	return result
}

func parseTime(timeStr string) (time.Time, error) {
	if len(timeStr) >= 14 {
		return time.Parse("20060102150405", timeStr[:14])
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
}

func formatTime(t time.Time) string {
	return t.Format("20060102150405")
}

func readCompleteElement(scanner *bufio.Scanner, firstLine, elementType string) string {
	block := firstLine
	endTag := "</" + elementType + ">"
	
	// Если первая строка уже содержит закрывающий тег, возвращаем ее
	if strings.Contains(firstLine, endTag) {
		return block
	}
	
	for scanner.Scan() {
		line := scanner.Text()
		block += "\n" + line
		if strings.Contains(line, endTag) {
			break
		}
	}
	
	return block
}

func extractChannel(line string) string {
	if idx := strings.Index(line, "channel=\""); idx != -1 {
		start := idx + 9
		rest := line[start:]
		if end := strings.Index(rest, "\""); end != -1 {
			return rest[:end]
		}
	}
	return ""
}

func getAttribute(xml string, attr string) string {
	pattern := attr + "=\""
	if idx := strings.Index(xml, pattern); idx != -1 {
		rest := xml[idx+len(pattern):]
		if endIdx := strings.Index(rest, "\""); endIdx != -1 {
			return rest[:endIdx]
		}
	}
	return ""
}

func extractTitleFromLine(line string) string {
	// Ищем начало тега title (может быть с атрибутами)
	startTag := "<title"
	startIdx := strings.Index(line, startTag)
	if startIdx == -1 {
		return ""
	}
	
	// Находим закрывающую скобку начала тега
	startTagEnd := strings.Index(line[startIdx:], ">")
	if startTagEnd == -1 {
		return ""
	}
	
	// Вычисляем позицию начала текста title
	titleStart := startIdx + startTagEnd + 1
	
	// Ищем закрывающий тег
	endTag := "</title>"
	endIdx := strings.Index(line[titleStart:], endTag)
	if endIdx == -1 {
		return ""
	}
	
	// Извлекаем текст title
	title := line[titleStart:titleStart+endIdx]
	return strings.TrimSpace(title)
}

func removeXMLEntities(s string) string {
	// Удаляем XML/HTML entities
	s = strings.ReplaceAll(s, "&amp;quot;", "")
	s = strings.ReplaceAll(s, "&quot;", "")
	s = strings.ReplaceAll(s, "&amp;amp;", "")
	s = strings.ReplaceAll(s, "&amp;", "")
	s = strings.ReplaceAll(s, "&amp;lt;", "")
	s = strings.ReplaceAll(s, "&lt;", "")
	s = strings.ReplaceAll(s, "&amp;gt;", "")
	s = strings.ReplaceAll(s, "&gt;", "")
	s = strings.ReplaceAll(s, "&amp;apos;", "")
	s = strings.ReplaceAll(s, "&apos;", "")
	
	// Удаляем числовые entities
	s = strings.ReplaceAll(s, "&#34;", "")
	s = strings.ReplaceAll(s, "&#38;", "")
	s = strings.ReplaceAll(s, "&#39;", "")
	s = strings.ReplaceAll(s, "&#60;", "")
	s = strings.ReplaceAll(s, "&#62;", "")
	
	return s
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}


