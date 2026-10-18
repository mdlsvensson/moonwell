namespace Moonwell.Testing
{
    /// <summary>Cases that check object ids registered without a maker, upgrade levels, and the register step of a start.</summary>
    public static class ObjectKindCases
    {
        /// <summary>A game component that logs "added" when the game adds it.</summary>
        private sealed class AddLogger : GameComponent
        {
            /// <summary>Logs the add.</summary>
            protected override void OnAdded()
            {
                TestLog.Lines.Add("added");
            }
        }

        /// <summary>A map that lists one component, AddLogger.</summary>
        private sealed class LoggingMap : Map
        {
            /// <summary>Lists the component that logs its add.</summary>
            public LoggingMap()
            {
                Components.Add(new AddLogger());
            }
        }

        /// <summary>The level a player has of an upgrade is the level set for the player's slot.</summary>
        public static void UpgradeLevel(FakeWorld world)
        {
            ObjectTypes.Id<TestForge>(ObjectTypes.FourCC("R001"));
            world.SetUpgradeLevel(0, ObjectTypes.FourCC("R001"), 2);
            Expect.Equal(2, Player.Get(0).LevelOf<TestForge>(), "the level of TestForge for player 0");
        }

        /// <summary>A second id for the same class throws.</summary>
        public static void IdTwice(FakeWorld world)
        {
            ObjectTypes.Id<TestForge>(ObjectTypes.FourCC("R001"));
            Expect.Throws<MoonwellException>(() => ObjectTypes.Id<TestForge>(ObjectTypes.FourCC("R002")), "a second id for TestForge");
        }

        /// <summary>Install runs the register step before the map's components are added.</summary>
        public static void RegisterBeforeComponents(FakeWorld world)
        {
            TestLog.Lines.Clear();
            Runtime.Install(new LoggingMap(), () => TestLog.Lines.Add("register"));
            Expect.Equal(2, TestLog.Lines.Count, "the log line count");
            Expect.Equal("register", TestLog.Lines[0], "the first log line");
            Expect.Equal("added", TestLog.Lines[1], "the second log line");
        }
    }
}
