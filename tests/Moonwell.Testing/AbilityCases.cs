using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check abilities and casts: the ability of a unit, the order of a cast's reactions, what a cast owns, and when a cast ends.</summary>
    public static class AbilityCases
    {
        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Registers the striker, the test bolt, a game component and a player component for player 0.</summary>
        private static void Arena()
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new Striker());
            ObjectTypes.Ability(ObjectTypes.FourCC("A000"), () => new TestBolt());
            Game.Instance.Add(new GameWatch());
            Player.Get(0).Add(new PlayerWatch("p0"));
        }

        /// <summary>A registered ability is an object of its class, owned by the unit, and a unit gives the same object again; an unregistered one is a plain Ability.</summary>
        public static void Registered(FakeWorld world)
        {
            ObjectTypes.Ability(ObjectTypes.FourCC("A000"), () => new TestBolt());
            var caster = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var bolt = caster.AbilityOf(ObjectTypes.FourCC("A000"));
            Expect.True(bolt is TestBolt, "the ability is a TestBolt");
            Expect.Same(caster, bolt.Unit, "the unit of the ability");
            Expect.Equal(ObjectTypes.FourCC("A000"), bolt.Id, "the id of the ability");
            Expect.Same(caster, bolt.Owner, "the owner of the ability");
            Expect.Same(bolt, caster.AbilityOf(ObjectTypes.FourCC("A000")), "the ability asked for again");
            Expect.Equal("Ability", Names.NameOf(caster.AbilityOf(ObjectTypes.FourCC("AHhb"))), "an unregistered ability");
        }

        /// <summary>AbilityOf with a class gives the object of that registered class.</summary>
        public static void Generic(FakeWorld world)
        {
            ObjectTypes.Ability(ObjectTypes.FourCC("A000"), () => new TestBolt());
            var caster = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            Expect.Same(caster.AbilityOf(ObjectTypes.FourCC("A000")), caster.AbilityOf<TestBolt>(), "the ability of the class");
        }

        /// <summary>A cast runs the ability's OnCast, then the caster's type and components, then the map's, then the caster's player's; Owner.Current in OnCast is the cast.</summary>
        public static void Order(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var bolt = striker.AbilityOf<TestBolt>();
            TestLog.Lines.Clear();
            Inbound.SpellEffect(striker.Handle, ObjectTypes.FourCC("A000"), null, 0f, 0f);
            ExpectLog(new string[] { "bolt cast", "striker cast", "striker comp cast", "game cast", "p0 cast" }, "the log");
            Expect.True(bolt.Given != null, "the cast was given to OnCast");
            Expect.Same(bolt.Given, bolt.Seen, "Owner.Current inside OnCast");
        }

        /// <summary>The cast has the level read when it begins, the target's object, and the given place.</summary>
        public static void Fields(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo"), slot: 1))!;
            var bolt = striker.AbilityOf<TestBolt>();
            world.SetAbilityLevel(striker.Handle, ObjectTypes.FourCC("A000"), 2);
            Inbound.SpellEffect(striker.Handle, ObjectTypes.FourCC("A000"), target.Handle, 3f, 4f);
            var cast = bolt.Given!;
            Expect.Equal(2, cast.Level, "the level of the cast");
            Expect.Same(target, cast.Target, "the target of the cast");
            Expect.Same(striker, cast.Caster, "the caster of the cast");
            Expect.Near(3f, cast.X, "the x of the cast");
            Expect.Near(4f, cast.Y, "the y of the cast");
        }

        /// <summary>A timer started in OnCast runs until the spell ends, then the cast's end stops it.</summary>
        public static void OwnsWhatItStarts(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var bolt = striker.AbilityOf<TestBolt>();
            Inbound.SpellEffect(striker.Handle, ObjectTypes.FourCC("A000"), null, 0f, 0f);
            world.Advance(0.6f);
            Expect.Equal(1, bolt.Ticks, "the ticks before the spell ends");
            Inbound.SpellEnded(striker.Handle, ObjectTypes.FourCC("A000"));
            Expect.True(bolt.Given!.HasEnded, "the cast has ended");
            var ticks = bolt.Ticks;
            world.Advance(2f);
            Expect.Equal(ticks, bolt.Ticks, "the ticks after the spell ends");
        }

        /// <summary>A second cast by the same caster ends the first one.</summary>
        public static void NextEndsPrevious(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var bolt = striker.AbilityOf<TestBolt>();
            Inbound.SpellEffect(striker.Handle, ObjectTypes.FourCC("A000"), null, 0f, 0f);
            var first = bolt.Given!;
            Inbound.SpellEffect(striker.Handle, ObjectTypes.FourCC("A000"), null, 0f, 0f);
            Expect.True(first.HasEnded, "the first cast has ended");
            Expect.True(first != bolt.Given, "the second cast is a new cast");
            Expect.True(!bolt.Given!.HasEnded, "the second cast is live");
        }

        /// <summary>Registering an ability with a reaction makes the world deliver the spell event, once.</summary>
        public static void Listens(FakeWorld world)
        {
            Expect.True(!world.Listening.Contains(GameEvent.Spell), "Spell is not listened for before a class needs it");
            ObjectTypes.Ability(ObjectTypes.FourCC("A000"), () => new TestBolt());
            Expect.True(world.Listening.Contains(GameEvent.Spell), "Spell is listened for");
        }
    }
}
