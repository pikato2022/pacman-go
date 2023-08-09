package main

// public: Capital
// private : lowercase
// import start
import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/danicat/simpleansi"
)

// import end
// global variable start
var (
	configFile = flag.String("config-file", "config.js", "path to config file")
	mazeFile   = flag.String("maze-file", "maze.txt", "path to maze file")
)
var maze []string

type sprite struct {
	row      int
	col      int
	startRow int
	startCol int
}
type ghost struct {
	position sprite
	status   GhostStatus
}
type Config struct { // need to be public for json decoder to work
	Player           string        `json:"player"`
	Ghost            string        `json:"ghost"`
	Wall             string        `json:"wall"`
	Dot              string        `json:"dot"`
	Pill             string        `json:"pill"`
	Death            string        `json:"death"`
	Space            string        `json:"space"`
	UseEmoji         bool          `json:"use_emoji"`
	GhostBlue        string        `json:"ghost_blue"`
	PillDurationSecs time.Duration `json:"pill_duration_secs"`
}
type GhostStatus string

const (
	GhostStatusNormal GhostStatus = "Normal"
	GhostStatusBlue   GhostStatus = "Blue"
)

var player sprite
var ghosts []*ghost // slice of pointer to sprite
var score int
var numDots int
var lives = 3
var cfg Config

// global variable end

func loadConfig(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return err
	}
	return nil
}

// read file
func loadMaze(file string) error {
	f, error := os.Open(file)
	if error != nil {
		log.Print("Some error happen when reading file")
		return error
	}
	defer f.Close() // defer so will run last
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		maze = append(maze, line)
	}
	for row, line := range maze {
		for col, char := range line {
			switch char {
			case 'P':
				player = sprite{row, col, row, col}

			case 'G':
				ghosts = append(ghosts, &ghost{sprite{row, col, row, col}, GhostStatusNormal})
			case '.':
				numDots++
			}
		}
	}
	// log.Println("Current player at ", player)
	return nil
}

func moveCursor(row, col int) {
	if cfg.UseEmoji {
		simpleansi.MoveCursor(row, col*2)
	} else {
		simpleansi.MoveCursor(row, col)
	}
}
func getLiveAsEmoji() string {
	buff := bytes.Buffer{}
	for i := lives; i > 0; i-- {
		buff.WriteString(cfg.Player)
	}
	return buff.String()
}
func printScreen() {
	simpleansi.ClearScreen()
	for _, line := range maze {
		for _, chr := range line {
			switch chr {
			case '#':
				fmt.Print(simpleansi.WithBlueBackground(cfg.Wall))
			case '.':
				fmt.Print(cfg.Dot)
			case 'X':
				fmt.Print(cfg.Pill)
			default:
				fmt.Printf(cfg.Space)

			}
		}
		fmt.Println()
	}
	ghostStatusMx.RLock()
	for _, g := range ghosts {
		moveCursor(g.position.row, g.position.col)
		if g.status == GhostStatusNormal {
			fmt.Printf(cfg.Ghost)
		} else if g.status == GhostStatusBlue {
			fmt.Printf(cfg.GhostBlue)
		}
	}
	ghostStatusMx.RUnlock()
	moveCursor(player.row, player.col)
	fmt.Print(cfg.Player)
	moveCursor(len(maze)+1, 0)
	liveRemaining := strconv.Itoa(lives)
	if cfg.UseEmoji {
		liveRemaining = getLiveAsEmoji()
	}
	fmt.Println("Score: ", score, "\nLives:", liveRemaining)
}
func drawDirection() string {
	dir := rand.Intn(4)
	move := map[int]string{
		0: "UP",
		1: "DOWN",
		2: "RIGHT",
		3: "LEFT",
	}
	return move[dir]
}
func moveGhosts() {
	for _, g := range ghosts {
		dir := drawDirection()
		g.position.row, g.position.col = makeMove(g.position.row, g.position.col, dir)
	}
}
func initialise() {
	cbTerm := exec.Command("stty", "cbreak", "-echo") // stty : change mode, cbreak: target mode,
	cbTerm.Stdin = os.Stdin
	err := cbTerm.Run()
	if err != nil {
		log.Fatalln("Unable to activate cbreak mode", err) // after this print, terminate program
	}
}
func cleanup() {
	cookedTerm := exec.Command("stty", "-cbreak", "echo") // '-' mean reverse
	cookedTerm.Stdin = os.Stdin
	err := cookedTerm.Run()
	if err != nil {
		log.Fatalln("Unable to restore cooked mode", err)
	}
}

func readInput() (string, error) {
	buffer := make([]byte, 100)
	cnt, err := os.Stdin.Read(buffer)
	if err != nil {
		log.Fatalln("Cant read input", err)
	}
	if cnt == 1 && buffer[0] == 0x1b {
		// log.Println("You press ESC")
		return "ESC", nil
	} else if cnt >= 1 {
		// if buffer[0] == 0x1b && buffer[1] == '[' {
		switch buffer[0] {
		case 'w':
			return "UP", nil
		case 's':
			return "DOWN", nil
		case 'd':
			return "RIGHT", nil
		case 'a':
			return "LEFT", nil
		}
		// }
	}

	return "", nil
}

func makeMove(oldRow, oldCol int, dir string) (newRow, newCol int) {
	newRow, newCol = oldRow, oldCol
	switch dir {
	case "UP":
		newRow--
		if newRow < 0 {
			newRow = 0
		}
	case "DOWN":
		newRow++
		if newRow == len(maze) {
			newRow = len(maze) - 1
		}
	case "RIGHT":
		newCol++
		if newCol == len(maze[0]) {
			newCol = len(maze[0]) - 1
		}
	case "LEFT":
		newCol--
		if newCol < 0 {
			newCol = 0
		}
		// fallthrough // to go to the next case
	}
	if maze[newRow][newCol] == '#' {
		newRow, newCol = oldRow, oldCol
	}
	return
}
func movePlayer(dir string) {
	player.row, player.col = makeMove(player.row, player.col, dir)
	removeDot := func(row, col int) {
		maze[row] = maze[row][0:col] + " " + maze[row][col+1:]
	}
	switch maze[player.row][player.col] {
	case '.':
		score++
		numDots--
		removeDot(player.row, player.col)
	case 'X':
		score += 10
		removeDot(player.row, player.col)
		go processPill()
	}

}

var pillTimer *time.Timer
var pillMx sync.Mutex

func processPill() {
	pillMx.Lock()
	updateGhosts(ghosts, GhostStatusBlue)
	if pillTimer != nil {
		pillTimer.Stop()
	}
	pillTimer = time.NewTimer(time.Second * cfg.PillDurationSecs)
	pillMx.Unlock()
	<-pillTimer.C
	pillMx.Lock()
	pillTimer.Stop()
	updateGhosts(ghosts, GhostStatusNormal)
	pillMx.Unlock()
}

var ghostStatusMx sync.RWMutex

func updateGhosts(ghosts []*ghost, ghostStatus GhostStatus) {
	ghostStatusMx.Lock()
	defer ghostStatusMx.Unlock()
	for _, g := range ghosts {
		g.status = ghostStatus
	}
}
func main() {
	flag.Parse()
	// init game
	initialise()
	defer cleanup()

	// load game
	err := loadMaze(*mazeFile)
	if err != nil {
		log.Println("Some Error happened")
		return
	}
	err = loadConfig(*configFile)
	if err != nil {
		log.Println("failed to load config ", err)
		return
	}

	// // process input
	input := make(chan string) // now input is channel
	go func(ch chan<- string) {
		for {
			input, err := readInput()
			if err != nil {
				log.Println("error reading input: ", err)
				ch <- "ESC"
			}
			ch <- input
		}
	}(input)
	// While true
	for {

		// process movement
		select {
		case inp := <-input:
			if inp == "ESC" {
				lives = 0
			}
			movePlayer(inp)
		default:
		}
		// movePlayer(input)
		moveGhosts()
		//process collision
		for _, g := range ghosts {

			if player.row == g.position.row && player.col == g.position.col {
				ghostStatusMx.RLock()
				if g.status == GhostStatusNormal {
					lives = lives - 1
					if lives != 0 {
						moveCursor(player.row, player.col)
						fmt.Print(cfg.Death)
						moveCursor(len(maze)+2, 0)
						ghostStatusMx.RUnlock()
						updateGhosts(ghosts, GhostStatusNormal)
						time.Sleep(1000 * time.Millisecond) //dramatic pause before reseting player position
						player.row, player.col = player.startRow, player.startCol
					}
				} else if g.status == GhostStatusBlue {
					ghostStatusMx.RUnlock()
					updateGhosts([]*ghost{g}, GhostStatusNormal)
					g.position.row, g.position.col = g.position.startRow, g.position.startCol
				}
			}

		}
		// update game state
		printScreen()

		// game end
		if numDots == 0 || lives <= 0 {
			if lives == 0 {
				moveCursor(player.row, player.col)
				fmt.Print(cfg.Death)
				moveCursor(len(maze)+2, 0)
			}
			if numDots == 0 {
				moveCursor(len(maze)+2, 0)
				fmt.Println("Win game")
			}
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}
