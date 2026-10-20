using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check what a player and the game show and hold: text on the screen, gold and food, who is playing.</summary>
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
