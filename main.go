package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/menocar/xmage/leaderboard/record"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/js"
)

var matchLog = flag.String("match_log", "", "")

type byEndTimeMsec []*record.Match

func (b byEndTimeMsec) Len() int           { return len(b) }
func (b byEndTimeMsec) Less(i, j int) bool { return b[i].GetEndTimeMsec() < b[j].GetEndTimeMsec() }
func (b byEndTimeMsec) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func read() *record.Matches {
	data, err := ioutil.ReadFile(*matchLog)
	if err != nil {
		log.Fatal(err)
	}
	matches := &record.Matches{}
	err = proto.Unmarshal(data, matches)
	if err != nil {
		log.Fatal(err)
	}
	sort.Sort(byEndTimeMsec(matches.GetMatches()))
	return matches
}

func Elo(ratingA, ratingB, scoreA, scoreB float64) (newRatingA, newRatingB float64) {
	expectedScoreA := 1.0 / (1.0 + math.Pow(10.0, (ratingB-ratingA)/400.0))
	expectedScoreB := 1.0 / (1.0 + math.Pow(10.0, (ratingA-ratingB)/400.0))
	const K = 16
	newRatingA = ratingA + K*(scoreA-expectedScoreA)
	newRatingB = ratingB + K*(scoreB-expectedScoreB)
	return
}

type Player struct {
	Name           string
	Draftbot       int
	Quit           int
	ComputerPlayer bool
	Bye            int
	Win            int
	Lose           int
	Matches        []*Match
	Rating         float64
	NLCMember      bool
}

type players map[string]*Player

func (ps players) getPlayer(name string) *Player {
	p, ok := ps[name]
	if !ok {
		p = &Player{
			Name:      name,
			Rating:    1500.0,
			NLCMember: true,
		}
		ps[name] = p
	}
	return p
}

type byRating []*Player

func (b byRating) Len() int           { return len(b) }
func (b byRating) Less(i, j int) bool { return b[i].Rating > b[j].Rating }
func (b byRating) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type Match struct {
	ID              int
	DeckTypeRaw     string
	DeckType        string
	DeckTypeComment string
	Players         []*Player
	PlayersRaw      string
	GameType        string
	Result          string
	Start           time.Time
	End             time.Time
	Results         []*MatchResult
}

type matches map[int]*Match

var deckTypeRegexp = regexp.MustCompile("([^\\[]+)(?:\\[(.+)\\])?")
var deckTypeReplaceRegexp = regexp.MustCompile("\\s*Limited\\s*")

func (ms matches) getMatch(mp *record.Match) *Match {
	id := int(mp.GetId())
	m, ok := ms[id]
	if !ok {
		dt := deckTypeRegexp.FindStringSubmatch(mp.GetDeckType())
		const (
			deckTypeIndex        = 1
			deckTypeCommentIndex = 2
		)
		deckType := dt[deckTypeIndex]
		if deckType == "" {
			log.Fatalf("deck type is empty (%s)", mp.GetDeckType())
		}
		m = &Match{
			ID:              id,
			DeckTypeRaw:     mp.GetDeckType(),
			DeckType:        deckTypeReplaceRegexp.ReplaceAllString(deckType, ""),
			DeckTypeComment: dt[deckTypeCommentIndex],
			PlayersRaw:      mp.GetPlayers(),
			GameType:        mp.GetGameType(),
			Result:          mp.GetResult(),
			Start:           time.Unix(0, mp.GetStartTimeMsec()*1000000),
			End:             time.Unix(0, mp.GetEndTimeMsec()*1000000),
		}
		ms[id] = m
	}
	return m
}

type byEnd []*Match

func (b byEnd) Len() int           { return len(b) }
func (b byEnd) Less(i, j int) bool { return b[i].End.After(b[j].End) }
func (b byEnd) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type PlayerResult struct {
	Player *Player

	Bye bool

	Wins         int
	Quit         bool
	Timeout      bool
	RatingChange float64
}

func (r *PlayerResult) String() string {
	var m string
	if r.Quit {
		m = "Q"
	} else if r.Timeout {
		m = "T"
	}
	return fmt.Sprintf("%d%s", r.Wins, m)
}

type MatchResult struct {
	Round    int
	Player   *PlayerResult
	Opponent *PlayerResult
}

func (r *MatchResult) String() string {
	ret := fmt.Sprintf("R%d ", r.Round)
	if r.Player.Bye {
		ret += fmt.Sprintf(" %s Bye", r.Player.Player.Name)
	} else {
		ret += fmt.Sprintf(" %s vs %s [%s-%s] - ", r.Player.Player.Name,
			r.Opponent.Player.Name, r.Player.String(), r.Opponent.String())
		if r.Win() {
			ret += r.Player.Player.Name
		} else {
			ret += r.Opponent.Player.Name
		}
		ret += " wins ("
		sp, so, ok := r.Score()
		if !ok {
			ret += "no-op"
		} else {
			var score, rating float64
			if r.Win() {
				score, rating = sp, r.Player.RatingChange
			} else {
				score, rating = so, r.Opponent.RatingChange
			}
			ret += fmt.Sprintf("%.2f, +%.2f", score, rating)
		}
		ret += ")"
	}
	return ret
}

func (r *MatchResult) Win() bool {
	if r.Player.Quit || r.Player.Timeout {
		return false
	}
	if r.Opponent.Quit || r.Opponent.Timeout {
		return true
	}
	return r.Player.Wins > r.Opponent.Wins
}

func (r *MatchResult) Score() (scoreP, scoreO float64, ok bool) {
	if r.Player.Quit || r.Opponent.Quit {
		// XMage disconnects a lot lately, safe to treat these as such.
		return 0, 0, false
	}
	p, o := float64(r.Player.Wins), float64(r.Opponent.Wins)
	return p / (p + o), o / (p + o), true
}

type byRound []*MatchResult

func (b byRound) Len() int           { return len(b) }
func (b byRound) Less(i, j int) bool { return b[i].Round < b[j].Round }
func (b byRound) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

var resultRegexp = regexp.MustCompile("(?:([a-zA-Z0-9_]+)|Draftbot \\(([a-zA-Z0-9_]+)\\)|(Computer Player [0-9]+)):( R[0-9]+ (?:(Bye)|[a-zA-Z0-9_]+ \\[[0-9]+[QT]?\\-[0-9]+[QT]?\\]))*")
var subResultRegexp = regexp.MustCompile("R([0-9]+) (?:(Bye)|([a-zA-Z0-9_]+) \\[([0-9]+)([A-Z])?\\-([0-9]+)([A-Z])?\\])")

func writeData(ps []*Player, ms []*Match) {
	t := template.Must(template.ParseFiles("./templates/data.js"))

	var b bytes.Buffer
	bio := bufio.NewWriter(&b)
	data := struct {
		Players     []*Player
		Matches     []*Match
		LastUpdated time.Time
	}{
		Players:     ps,
		Matches:     ms,
		LastUpdated: time.Now(),
	}
	err := t.Execute(bio, data)
	if err != nil {
		log.Fatal(err)
	}
	err = bio.Flush()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile("./leaderboard/data.js", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	js.Minify(minify.New(), f, bytes.NewReader(b.Bytes()), nil)
}

func toInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func init() {
	flag.Parse()
}

func main() {
	ps := make(players)
	ms := make(matches)
	for _, matchProto := range read().GetMatches() {
		if !strings.HasPrefix(matchProto.GetGameType(), "Booster Draft") {
			continue
		}

		m := ms.getMatch(matchProto)
		rs := resultRegexp.FindAllStringSubmatch(matchProto.GetResult(), -1)
		for _, r := range rs {
			const (
				allIndex            = 0
				nameIndex           = 1
				draftbotIndex       = 2
				computerPlayerIndex = 3
				resultIndex         = 4
			)
			var p *Player
			if name := r[nameIndex]; name != "" {
				p = ps.getPlayer(name)
				p.NLCMember = true
			} else if name := r[draftbotIndex]; name != "" {
				p = ps.getPlayer(name)
				p.Draftbot++
				p.NLCMember = false
			} else if name := r[computerPlayerIndex]; name != "" {
				p = ps.getPlayer(name)
				p.ComputerPlayer = true
			}
			p.Matches = append(p.Matches, m)
			m.Players = append(m.Players, p)
			for _, sr := range subResultRegexp.FindAllStringSubmatch(r[allIndex], -1) {
				const (
					roundIndex          = 1
					byeIndex            = 2
					opponentIndex       = 3
					winIndex            = 4
					matchDetailIndex    = 5 // "T/Q"
					oppWinIndex         = 6
					oppMatchDetailIndex = 7
				)
				mr := &MatchResult{
					Round: toInt(sr[roundIndex]),
				}
				if sr[byeIndex] == "Bye" {
					p.Bye++
					mr.Player = &PlayerResult{
						Player: p,
						Bye:    true,
					}
				} else {
					pd := sr[matchDetailIndex]
					mr.Player = &PlayerResult{
						Player:  p,
						Wins:    toInt(sr[winIndex]),
						Quit:    pd == "Q",
						Timeout: pd == "T",
					}
					od := sr[oppMatchDetailIndex]
					mr.Opponent = &PlayerResult{
						Player:  ps.getPlayer(sr[opponentIndex]),
						Wins:    toInt(sr[oppWinIndex]),
						Quit:    od == "Q",
						Timeout: od == "T",
					}
					if mr.Win() {
						p.Win++
					} else {
						p.Lose++
					}
					if mr.Player.Quit {
						p.Quit++
					}
				}
				// Add a same result only once.
				if mr.Opponent == nil || mr.Player.Player.Name < mr.Opponent.Player.Name {
					m.Results = append(m.Results, mr)
				}
			}
		}
		sort.Sort(byRound(m.Results))
		for _, r := range m.Results {
			p, o := r.Player, r.Opponent
			if p.Bye {
				continue
			}
			if p.Player.ComputerPlayer || o.Player.ComputerPlayer {
				// Do not compute rating for computer players.
				continue
			}
			sp, so, ok := r.Score()
			if !ok {
				continue
			}
			rp, ro := p.Player.Rating, o.Player.Rating
			p.Player.Rating, o.Player.Rating = Elo(rp, ro, sp, so)
			p.RatingChange = p.Player.Rating - rp
			o.RatingChange = o.Player.Rating - ro
		}
	}
	var pslice []*Player
	for _, p := range ps {
		pslice = append(pslice, p)
	}
	sort.Sort(byRating(pslice))

	var mslice []*Match
	for _, m := range ms {
		mslice = append(mslice, m)
	}
	sort.Sort(byEnd(mslice))

	writeData(pslice, mslice)
}
