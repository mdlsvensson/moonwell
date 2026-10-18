namespace Moonwell.Testing
{
    /// <summary>Cases that check the map class: its components start in list order, they are not added before the start, and its settings default to empty.</summary>
    public static class MapCases
    {
        /// <summary>Install adds the map's components to the game in list order, and each one's OnAdded runs once.</summary>
        public static void ComponentsInOrder(FakeWorld world)
        {
            var map = new TestMap();
            Runtime.Install(map, () => { });
            var game = Game.Instance.GetAll<GameComponent>();
            Expect.Equal(2, game.Count, "the game's components");
            Expect.Same(map.StartComponents[0], game[0], "the first component of the game");
            Expect.Same(map.StartComponents[1], game[1], "the second component of the game");
            Expect.Equal(1, ((TestGameA)game[0]).Adds, "OnAdded runs of TestGameA");
            Expect.Equal(1, ((TestGameB)game[1]).Adds, "OnAdded runs of TestGameB");
        }

        /// <summary>Right after a map is constructed, the game has no components and no component's OnAdded has run.</summary>
        public static void ComponentsNotAddedEarly(FakeWorld world)
        {
            var map = new TestMap();
            Expect.Equal(0, Game.Instance.ComponentCount, "the game's components after construction");
            Expect.Equal(0, ((TestGameA)map.StartComponents[0]).Adds, "OnAdded runs of TestGameA after construction");
            Expect.Equal(0, ((TestGameB)map.StartComponents[1]).Adds, "OnAdded runs of TestGameB after construction");
        }

        /// <summary>A map that sets no balance reads Balance.Default.</summary>
        public static void BalanceDefault(FakeWorld world)
        {
            var map = new TestMap();
            Expect.True(map.Balance == Balance.Default, "Balance");
        }

        /// <summary>A map that sets none of its settings reads an empty string for each of the four.</summary>
        public static void SettingsDefaultEmpty(FakeWorld world)
        {
            var map = new TestMap();
            Expect.Equal("", map.Name, "Name");
            Expect.Equal("", map.Author, "Author");
            Expect.Equal("", map.Description, "Description");
            Expect.Equal("", map.RecommendedPlayers, "RecommendedPlayers");
        }
    }
}
