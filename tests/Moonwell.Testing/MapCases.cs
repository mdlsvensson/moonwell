using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check the map class: its components start in list order, they are not added before the start, and its settings default to empty; and the runtime's begin, which takes in what the map placed.</summary>
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
            Unit.Create(ObjectTypes.FourCC("h0df"), Player.One, 0f, 0f, scope: Game.Instance);
            Expect.True(world.LastFacing == 270f, "the facing of Create with an owner");
        }

        /// <summary>
        /// Runtime.Begin, which Start runs on the game's first timer: after the setup has registered the types, every unit already
        /// in the world is taken in, and each item and destructable of a registered type, and only then does the game start.
        /// </summary>
        public static void BeginTakesInPlaced(FakeWorld world)
        {
            var striker = world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0);
            var footman = world.NewUnit(ObjectTypes.FourCC("hfoo"), slot: 1);
            var orb = world.NewItem(ObjectTypes.FourCC("I000"));
            var claws = world.NewItem(ObjectTypes.FourCC("ratc"));
            var gate = world.NewDestructable(ObjectTypes.FourCC("B000"));
            var tree = world.NewDestructable(ObjectTypes.FourCC("LTlt"));
            world.PlacedUnits.Add(striker);
            world.PlacedUnits.Add(footman);
            world.PlacedItems.Add(orb);
            world.PlacedItems.Add(claws);
            world.PlacedDestructables.Add(gate);
            world.PlacedDestructables.Add(tree);
            var atStart = new List<string>();
            Runtime.Begin(() =>
            {
                ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new Striker());
                ObjectTypes.Item(ObjectTypes.FourCC("I000"), () => new TestOrb());
                ObjectTypes.Destructable(ObjectTypes.FourCC("B000"), () => new TestGate());
                Game.Instance.Add(new StartRunner(() =>
                {
                    atStart.Add((KnownUnits.Find(striker) != null) + " " + (KnownItems.Find(orb) != null) + " " + (KnownDestructables.Find(gate) != null));
                }));
            });
            Expect.True(KnownUnits.Find(striker) is Striker, "the placed unit of a registered type is taken in as its class");
            Expect.True(KnownUnits.Find(footman) != null, "a placed unit of no registered type is taken in too");
            Expect.True(KnownItems.Find(orb) is TestOrb, "the placed item of a registered type is taken in as its class");
            Expect.True(KnownItems.Find(claws) == null, "a placed item of no registered type is not");
            Expect.True(KnownDestructables.Find(gate) is TestGate, "the placed destructable of a registered type is taken in as its class");
            Expect.True(KnownDestructables.Find(tree) == null, "a placed destructable of no registered type is not");
            Expect.Equal(1, atStart.Count, "the runs of OnGameStart");
            Expect.Equal("True True True", atStart[0], "what OnGameStart found taken in: the unit, the item, the destructable");
        }

        /// <summary>A game component that runs an action at the game's start.</summary>
        private sealed class StartRunner : GameComponent
        {
            private readonly Action run;

            /// <summary>Makes a component that runs the action at the game's start.</summary>
            public StartRunner(Action run)
            {
                this.run = run;
            }

            /// <summary>Runs the action.</summary>
            public override void OnGameStart()
            {
                run();
            }
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
