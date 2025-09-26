package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

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

	// Обрабатываем файл построчно
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
				// Создаем упрощенный programme только с title
				simpleProgramme := createSimpleProgramme(scanner, trimmed)
				if simpleProgramme != "" {
					writer.WriteString(simpleProgramme + "\n")
					programmesFound++
					if programmesFound%1000 == 0 {
						fmt.Printf("Processed %d programmes...\n", programmesFound)
					}
				}
			}
			continue
		}
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
	
	return scanner.Err()
}

func readCompleteElement(scanner *bufio.Scanner, firstLine, elementType string) string {
	block := firstLine + "\n"
	endTag := "</" + elementType + ">"
	
	for scanner.Scan() {
		line := scanner.Text()
		block += line + "\n"
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

func createSimpleProgramme(scanner *bufio.Scanner, firstLine string) string {
	start := getAttribute(firstLine, "start")
	stop := getAttribute(firstLine, "stop")
	channel := extractChannel(firstLine)
	
	if start == "" || stop == "" || channel == "" {
		return ""
	}
	
	// Ищем title в следующих строках
	title := "No title"
	linesToRead := 10 // Ограничим поиск title 10 строками
	
	// Сканируем следующие строки для поиска title
	tempScanner := bufio.NewScanner(strings.NewReader(firstLine + "\n"))
	
	for i := 0; i < linesToRead; i++ {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		tempScanner = bufio.NewScanner(strings.NewReader(tempScanner.Text() + "\n" + line))
		
		// Проверяем текущую строку на наличие title
		if strings.Contains(line, "<title>") {
			if foundTitle := extractTitleFromLine(line); foundTitle != "" {
				title = foundTitle
			}
		}
		
		if strings.Contains(line, "</programme>") {
			break
		}
	}
	
	// Экранируем специальные XML символы в title
	title = escapeXML(title)
	
	return fmt.Sprintf("<programme start=\"%s\" stop=\"%s\" channel=\"%s\">\n<title>%s</title>\n</programme>", 
		start, stop, channel, title)
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
	if idx := strings.Index(line, "<title>"); idx != -1 {
		start := idx + 7
		rest := line[start:]
		if end := strings.Index(rest, "</title>"); end != -1 {
			title := rest[:end]
			return strings.TrimSpace(title)
		}
	}
	return ""
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
