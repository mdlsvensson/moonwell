using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check what a player and the game show and hold: text on the screen, gold and food, who is playing, names, colours, teams, alliances and tech.</summary>
    public static class PlayerCases
    {
        /// <summary>A game component that logs who left.</summary>
        private class GameLeave : GameComponent
        {
            /// <summary>Logs the slot of the player who left.</summary>
            public override void OnLeave(Player player)
            {
                TestLog.Lines.Add("game: " + player.Slot);
            }
        }

        /// <summary>A player component that logs who left.</summary>
        private class PlayerLeave : PlayerComponent
        {
            /// <summary>Logs the slot of the player who left.</summary>
            public override void OnLeave(Player player)
            {
                TestLog.Lines.Add("player " + Player.Slot + ": " + player.Slot);
            }
        }

        /// <summary>Player.Playing holds the slots that are playing, in order; Player.Users only those a person controls.</summary>
        public static void PlayersPlaying(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Empty, Controller.User);
            world.SetSlot(2, SlotState.Playing, Controller.Computer);
            world.SetSlot(3, SlotState.Left, Controller.User);

            var playing = Player.Playing;
            Expect.Equal(2, playing.Count, "players playing");
            Expect.Same(Player.One, playing[0], "the first");
            Expect.Same(Player.Three, playing[1], "the second");
            var users = Player.Users;
            Expect.Equal(1, users.Count, "users playing");
            Expect.Same(Player.One, users[0], "the user");
        }

        /// <summary>A player's slot state and controller are the game's.</summary>
        public static void PlayersSlotState(FakeWorld world)
        {
            world.SetSlot(4, SlotState.Left, Controller.Computer);

            Expect.True(Player.Five.SlotState == SlotState.Left, "the slot state");
            Expect.True(Player.Five.Controller == Controller.Computer, "the controller");
            Expect.True(Player.Six.SlotState == SlotState.Empty, "a slot not set");
        }

        /// <summary>When a player leaves, the game's components react first, then the leaving player's.</summary>
        public static void PlayersLeaveOrder(FakeWorld world)
        {
            TestLog.Lines.Clear();
            Player.Two.Add(new PlayerLeave());
            Player.One.Add(new PlayerLeave());
            Game.Instance.Add(new GameLeave());

            Inbound.Left(world.PlayerAt(1));

            Expect.Equal(2, TestLog.Lines.Count, "reactions run");
            Expect.Equal("game: 1", TestLog.Lines[0], "the game's first");
            Expect.Equal("player 1: 1", TestLog.Lines[1], "then the leaving player's");
        }

        /// <summary>Leaving is listened for only once a component reacts to it.</summary>
        public static void PlayersLeaveListenedWhenReacted(FakeWorld world)
        {
            Expect.True(!world.Listening.Contains(GameEvent.Leave), "not listened for before a reaction needs it");
            Game.Instance.Add(new GameLeave());
            Expect.True(world.Listening.Contains(GameEvent.Leave), "listened for");
        }

        /// <summary>A player's Print shows the line to that player, for the game's own time.</summary>
        public static void PlayersPrint(FakeWorld world)
        {
            Player.Two.Print("Kills: 3");

            Expect.Equal(1, world.PrintedTexts.Count, "lines printed");
            Expect.Equal(1, world.PrintedSlots[0], "the slot printed to");
            Expect.Equal("Kills: 3", world.PrintedTexts[0], "the line");
            Expect.True(world.PrintedSeconds[0] < 0f, "the game's own time");
        }

        /// <summary>A player's Print with seconds shows the line for that long.</summary>
        public static void PlayersPrintSeconds(FakeWorld world)
        {
            Player.One.Print("Wave 2", 5f);

            Expect.Equal(0, world.PrintedSlots[0], "the slot printed to");
            Expect.True(world.PrintedSeconds[0] == 5f, "the seconds");
        }

        /// <summary>A player's gold is the game's: set, read and added to.</summary>
        public static void PlayersGold(FakeWorld world)
        {
            var player = Player.Two;
            player.Gold = 1000;
            player.Gold += 25;

            Expect.Equal(1025, player.Gold, "gold");
            Expect.Equal(1025, world.StateOf(world.PlayerAt(1), PlayerState.Gold), "the game's gold");
            Expect.Equal(0, Player.One.Gold, "another player's gold");
        }

        /// <summary>Each player state property reads and sets its own state.</summary>
        public static void PlayersStates(FakeWorld world)
        {
            var player = Player.One;
            player.Lumber = 1;
            player.FoodUsed = 2;
            player.FoodCap = 3;
            player.FoodCapCeiling = 4;
            player.GoldGathered = 5;
            player.LumberGathered = 6;
            player.GoldUpkeepRate = 7;
            player.LumberUpkeepRate = 8;

            var handle = world.PlayerAt(0);
            Expect.Equal(1, world.StateOf(handle, PlayerState.Lumber), "Lumber");
            Expect.Equal(2, world.StateOf(handle, PlayerState.FoodUsed), "FoodUsed");
            Expect.Equal(3, world.StateOf(handle, PlayerState.FoodCap), "FoodCap");
            Expect.Equal(4, world.StateOf(handle, PlayerState.FoodCapCeiling), "FoodCapCeiling");
            Expect.Equal(5, world.StateOf(handle, PlayerState.GoldGathered), "GoldGathered");
            Expect.Equal(6, world.StateOf(handle, PlayerState.LumberGathered), "LumberGathered");
            Expect.Equal(7, world.StateOf(handle, PlayerState.GoldUpkeepRate), "GoldUpkeepRate");
            Expect.Equal(8, world.StateOf(handle, PlayerState.LumberUpkeepRate), "LumberUpkeepRate");
            Expect.Equal(8, player.LumberUpkeepRate, "a read");
            Expect.Equal(0, player.Gold, "Gold is apart");
        }

        /// <summary>A player's name is the game's: read and set.</summary>
        public static void PlayersName(FakeWorld world)
        {
            Expect.Equal("Player 2", Player.Two.Name, "the game's name");
            Player.Two.Name = "Arthas";

            Expect.Equal("Arthas", Player.Two.Name, "the new name");
            Expect.Equal("Arthas", world.NameOf(world.PlayerAt(1)), "the game's new name");
            Expect.Equal("Player 1", Player.One.Name, "another player's name");
        }

        /// <summary>A player's colour is the game's: the slot's own until set.</summary>
        public static void PlayersColor(FakeWorld world)
        {
            Expect.True(Player.Two.Color == PlayerColor.Blue, "slot 1 starts blue");
            Player.Two.Color = PlayerColor.Peanut;

            Expect.True(Player.Two.Color == PlayerColor.Peanut, "the new colour");
            Expect.Equal(23, world.ColorOf(world.PlayerAt(1)), "the game's new colour");
            Expect.True(Player.One.Color == PlayerColor.Red, "another player's colour");
        }

        /// <summary>A player's team is the game's: read and set.</summary>
        public static void PlayersTeam(FakeWorld world)
        {
            Player.Three.Team = 1;

            Expect.Equal(1, Player.Three.Team, "the new team");
            Expect.Equal(1, world.TeamOf(world.PlayerAt(2)), "the game's team");
            Expect.Equal(0, Player.One.Team, "another player's team");
        }

        /// <summary>A player's race is the game's.</summary>
        public static void PlayersRace(FakeWorld world)
        {
            world.SetRace(1, Race.NightElf);

            Expect.True(Player.Two.Race == Race.NightElf, "the race");
            Expect.True(Player.One.Race == Race.Human, "a race not set");
        }

        /// <summary>An alliance goes one way, as the game keeps it, and decides who is an ally.</summary>
        public static void PlayersAlliance(FakeWorld world)
        {
            Player.One.SetAlliance(Player.Two, Alliance.Allied | Alliance.SharedVision);

            Expect.True(Player.One.AllianceWith(Player.Two) == (Alliance.Allied | Alliance.SharedVision), "one's alliance with two");
            Expect.True(Player.Two.AllianceWith(Player.One) == Alliance.None, "two's alliance with one");
            Expect.True(Player.One.IsAlly(Player.Two), "one is two's ally");
            Expect.True(!Player.One.IsEnemy(Player.Two), "one is not two's enemy");
            Expect.True(!Player.Two.IsAlly(Player.One), "two is not one's ally");
            Expect.True(Player.Two.IsEnemy(Player.One), "two is one's enemy");

            Player.One.SetAlliance(Player.Two, Alliance.None);
            Expect.True(Player.One.AllianceWith(Player.Two) == Alliance.None, "the alliance taken back");
        }

        /// <summary>Allied is blizzard.j's "allied": passive, both helps, shared experience and spells.</summary>
        public static void PlayersAllied(FakeWorld world)
        {
            Expect.True(Alliance.Allied == (Alliance.Passive | Alliance.HelpRequest | Alliance.HelpResponse | Alliance.SharedXp | Alliance.SharedSpells), "Allied");
        }

        /// <summary>An upgrade's level, and how far a unit type or an upgrade is allowed, are the game's.</summary>
        public static void PlayersTech(FakeWorld world)
        {
            ObjectTypes.Id<TestForge>(ObjectTypes.FourCC("R001"));
            ObjectTypes.Id<TestHero>(ObjectTypes.FourCC("H001"));

            Player.One.SetLevel<TestForge>(2);
            Expect.Equal(2, Player.One.LevelOf<TestForge>(), "the upgrade's level");
            Expect.Equal(0, Player.Two.LevelOf<TestForge>(), "another player's level");

            Expect.Equal(int.MaxValue, Player.One.MaxAllowed<TestHero>(), "no limit until set");
            Player.One.SetMaxAllowed<TestHero>(1);
            Player.One.SetMaxAllowed<TestForge>(0);
            Expect.Equal(1, Player.One.MaxAllowed<TestHero>(), "the unit's limit");
            Expect.Equal(0, Player.One.MaxAllowed<TestForge>(), "the upgrade's limit");
            Expect.Equal(1, world.TechMaxOf(world.PlayerAt(0), ObjectTypes.FourCC("H001")), "the game's limit");
        }

        /// <summary>An ability is made available or not for a player.</summary>
        public static void PlayersAbilityAvailable(FakeWorld world)
        {
            ObjectTypes.Id<TestBolt>(ObjectTypes.FourCC("A001"));

            Player.Two.SetAvailable<TestBolt>(false);

            Expect.Equal(1, world.AbilityCalls.Count, "calls");
            Expect.Equal("available 1 " + ObjectTypes.FourCC("A001") + " off", world.AbilityCalls[0], "the call");
        }

        /// <summary>The game's Print shows the line to everyone.</summary>
        public static void GamePrint(FakeWorld world)
        {
            Game.Print("Wave 2 is coming");

            Expect.Equal(1, world.PrintedTexts.Count, "lines printed");
            Expect.Equal(-1, world.PrintedSlots[0], "everyone");
            Expect.Equal("Wave 2 is coming", world.PrintedTexts[0], "the line");
        }
    }
}
