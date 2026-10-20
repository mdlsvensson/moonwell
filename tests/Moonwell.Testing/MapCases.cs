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

        /// <summary>A map that assigns its components starts the assigned list, in its order, and nothing it held before.</summary>
        public static void ComponentsAssigned(FakeWorld world)
        {
            Runtime.Install(new TestMapAssigned(), () => { });
            var game = Game.Instance.GetAll<GameComponent>();
            Expect.Equal(2, game.Count, "the game's components");
            Expect.True(game[0] is TestGameB, "TestGameB first");
            Expect.True(game[1] is TestGameA, "TestGameA second");
        }

        /// <summary>A map that adds several components in one call starts them in the order given.</summary>
        public static void ComponentsAddMany(FakeWorld world)
        {
            Runtime.Install(new TestMapAddMany(), () => { });
            var game = Game.Instance.GetAll<GameComponent>();
            Expect.Equal(3, game.Count, "the game's components");
            Expect.True(game[0] is TestGameA, "TestGameA first");
            Expect.True(game[1] is TestGameB, "TestGameB second");
            Expect.True(game[2] is TestGameA, "TestGameA third");
        }

        /// <summary>A unit class that assigns its components drops the ones its parent class listed.</summary>
        public static void SubclassAssigningDropsParent(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h0as"), () => new DummyAssigned());
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h0as")))!;
            Expect.Equal(1, unit.ComponentCount, "the unit's components");
            Expect.True(unit.Has<Tally>(), "the Tally");
            Expect.True(!unit.Has<DummyComp>(), "no DummyComp");
        }

        /// <summary>A unit class that adds several components in one call keeps the ones its parent class listed, first.</summary>
        public static void SubclassAddManyKeepsParent(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h0am"), () => new DummyAddMany());
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h0am")))!;
            var all = unit.GetAll<UnitComponent>();
            Expect.Equal(3, all.Count, "the unit's components");
            Expect.True(all[0] is DummyComp, "DummyComp first");
            Expect.True(all[1] is Tally, "Tally second");
            Expect.True(all[2] is StrikerComp, "StrikerComp third");
        }

        /// <summary>Unit.Create without a facing faces the game's default, 270 degrees, with or without an owner.</summary>
        public static void CreateDefaultFacing(FakeWorld world)
        {
            ObjectTypes.Id<Dummy>(ObjectTypes.FourCC("h0df"));
            ObjectTypes.Unit(ObjectTypes.FourCC("h0df"), () => new Dummy());
            Unit.Create<Dummy>(Player.One, 0f, 0f);
            Expect.True(world.LastFacing == 270f, "the facing of Create<T>");
            world.LastFacing = 0f;
            Unit.Create(ObjectTypes.FourCC("h0df"), Player.One, 0f, 0f, owner: Game.Instance);
            Expect.True(world.LastFacing == 270f, "the facing of Create with an owner");
        }

        /// <summary>Right after a map is constructed, the game has no components and no component's OnAdded has run.</summary>
        public static void ComponentsNotAddedEarly(FakeWorld world)
        {
            var map = new TestMap();
            Expect.Equal(0, Game.Instance.ComponentCount, "the game's components after construction");
            Expect.Equal(0, ((TestGameA)map.StartComponents[0]).Adds, "OnAdded runs of TestGameA after construction");
            Expect.Equal(0, ((TestGameB)map.StartComponents[1]).Adds, "OnAdded runs of TestGameB after construction");
        }

        /// <summary>A map that sets no game data option reads DataSet.Default and DataVersion.ForsakenKingdom.</summary>
        public static void BalanceDefault(FakeWorld world)
        {
            var map = new TestMap();
            Expect.True(map.DataSet == DataSet.Default, "DataSet");
            Expect.True(map.DataVersion == DataVersion.ForsakenKingdom, "DataVersion");
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
