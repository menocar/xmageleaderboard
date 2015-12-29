window.Data = (function() {
  var data = {};
  data.Players = [
    {{range $i, $_ := .Players}}{
      "name": "{{.Name}}",
      "rating": {{printf "%.0f" .Rating}},
      "matches": [{{range .Matches}}{{.ID}},{{end}}],
      "win": {{.Win}},
      "lose": {{.Lose}},
      "bye": {{.Bye}},
      "draftbot": {{.Draftbot}},
      "quit": {{.Quit}},
      {{if .NLCMember}}"nlc": true,{{end}}
    }, {{end}}
  ];

  data.PlayersByName = {};
  for (var i = 0; i < data.Players.length; ++i) {
      var p = data.Players[i];
      data.PlayersByName[p.name] = p;
  }

  data.Matches = [
    {{range .Matches}}{
      "id": {{.ID}},
      "deck_type": "{{.DeckType}}",
      "players": "{{.PlayersRaw}}",
      "game_type": "{{.GameType}}",
      "result": "{{.Result}}",
      "start_time": {{.Start.Unix}},
      "end_time": {{.End.Unix}},
      "results": [
        {{range .Results}}{
          "round": {{.Round}},
          {{if .Player.Bye}}
          "player": {
            "name": "{{.Player.Player.Name}}",
            "bye": true,
          },
          {{else}}
          "player": {
            "name": "{{.Player.Player.Name}}",
            "score": "{{.Player.String}}",
            "rating": {{.Player.RatingChange}},
          },
          "opponent": {
            "name": "{{.Opponent.Player.Name}}",
            "score": "{{.Opponent.String}}",
            "rating": {{.Opponent.RatingChange}},
          },
          {{end}}
        },
        {{end}}
      ],
    },
    {{end}}
  ];

  data.MatchesByID = {};
  for (var i = 0; i < data.Matches.length; ++i) {
    var m = data.Matches[i];
    data.MatchesByID[m.id] = m;
  }

  data.LastUpdated = {{.LastUpdated.Unix}};

  return data;
})();
