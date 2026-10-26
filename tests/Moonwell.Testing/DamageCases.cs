using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check damage: its order of reactions, the amount a reaction changes, a damage inside a damage, and removal mid-dispatch.</summary>
    public static class DamageCases
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

        /// <summary>Registers the striker and the dummy, a game component and a player component for each of the two players.</summary>
        private static void Arena()
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new Striker());
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Game.Instance.Add(new GameWatch());
            Player.Get(0).Add(new PlayerWatch("p0"));
            Player.Get(1).Add(new PlayerWatch("p1"));
        }

        /// <summary>Damage runs the source's type and components, then the target's, the game's, then the target's player and the source's player.</summary>
        public static void Order(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Near(100f, amount, "the amount");
        }

        /// <summary>A target component that halves the damage makes the call return the halved amount.</summary>
        public static void ChangeAmount(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new Halver());
            Expect.Near(50f, Inbound.Damaged(dummy.Handle, null, 100f, true), "the amount after the halving");
        }

        /// <summary>With no reaction anywhere, damage between units the runtime does not hold returns the amount and makes no objects.</summary>
        public static void UnknownUnits(FakeWorld world)
        {
            var target = world.NewUnit(ObjectTypes.FourCC("ofoo"));
            var source = world.NewUnit(ObjectTypes.FourCC("ofoo"));
            Expect.Near(100f, Inbound.Damaged(target, source, 100f, true), "the amount");
            Expect.True(KnownUnits.Find(target) == null, "the target is not made");
            Expect.True(KnownUnits.Find(source) == null, "the source is not made");
        }

        /// <summary>Unit.Damage asks the game to deal the damage with the game's numbers: magic, spells and no sound unless given.</summary>
        public static void DealFromCode(FakeWorld world)
        {
            var source = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 0))!;
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 1))!;
            var plain = source.Damage(target, 10f);
            Expect.True(plain != null, "the damage was dealt");
            Expect.Equal(1, world.Dealt.Count, "the damage the game was asked to deal");
            Expect.Equal("10 attack:False ranged:False 0 14 0", world.Dealt[0], "the plain damage's numbers");
            source.Damage(target, 5f, DamageKind.Fire, AttackType.Siege, isAttack: true, ranged: true, sound: WeaponSound.MetalLightChop);
            Expect.Equal("5 attack:True ranged:True 3 8 1", world.Dealt[1], "the given numbers");
        }

        /// <summary>Damage dealt from code runs the reactions inside the call, and returns the damage they saw.</summary>
        public static void DealRunsReactions(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new Halver());
            TestLog.Lines.Clear();
            var damage = striker.Damage(dummy, 100f);
            Expect.True(TestLog.Lines.Contains("dummy damaged"), "the target's reaction ran");
            Expect.Same(dummy, damage!.Target, "the damage's target");
            Expect.Same(striker, damage.Source, "the damage's source");
            Expect.Near(50f, damage.Amount, "the amount after the reactions");
        }

        /// <summary>Damage dealt from code as an attack reads as one in the reactions, although the game's event says it is not.</summary>
        public static void DealIsAttack(FakeWorld world)
        {
            var watch = new AttackWatch();
            Game.Instance.Add(watch);
            var source = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 0))!;
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 1))!;
            source.Damage(target, 5f, isAttack: true);
            source.Damage(target, 5f);
            Expect.Equal("attack|no attack", string.Join("|", watch.Seen), "what the reactions saw");
        }

        /// <summary>When the game deals nothing, Unit.Damage returns null.</summary>
        public static void DealRefused(FakeWorld world)
        {
            var source = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 0))!;
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 1))!;
            world.RefuseDamage = true;
            Expect.True(source.Damage(target, 10f) == null, "nothing was dealt");
        }

        /// <summary>The world is asked to deliver damage only once a class overrides OnDamaged.</summary>
        public static void Listen(FakeWorld world)
        {
            Expect.True(!world.Listening.Contains(GameEvent.Damage), "Damage is not listened for before a class needs it");
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Expect.True(world.Listening.Contains(GameEvent.Damage), "Damage is listened for");
        }

        /// <summary>A damage dealt from inside OnDamaged runs fully inside the call, and the outer damage keeps its own amount.</summary>
        public static void Nested(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Get<DummyComp>()!.Reply = striker.Handle;
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p0 damaged", "p1 damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Near(100f, amount, "the outer amount");
        }

        /// <summary>A reaction that removes its own unit stops the unit's later reactions, nothing is reported, and the map's reactions still run.</summary>
        public static void ActorEndsMidDispatch(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new Remover());
            dummy.Add(new B());
            TestLog.Lines.Clear();
            var amount = Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p1 damaged", "p0 damaged" }, "the log");
            Expect.Equal(0, world.Logged.Count, "the lines logged");
            Expect.True(dummy.HasEnded, "the dummy has ended");
            Expect.Near(100f, amount, "the amount");
        }

        /// <summary>Damage between two units of one player runs that player's reactions once.</summary>
        public static void SamePlayer(FakeWorld world)
        {
            Arena();
            var striker = Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 0))!;
            TestLog.Lines.Clear();
            Inbound.Damaged(dummy.Handle, striker.Handle, 100f, true);
            ExpectLog(new string[] { "striker dealt", "striker comp dealt", "dummy damaged", "dummy comp damaged", "game damaged", "p0 damaged" }, "the log");
        }

        /// <summary>The tag given to Unit.Damage is the damage's Tag inside the reactions, as well as on the damage it returns.</summary>
        public static void DealTagInReactions(FakeWorld world)
        {
            var watch = new TagWatch();
            Game.Instance.Add(watch);
            var source = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 0))!;
            var target = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo"), slot: 1))!;
            var tag = new object();
            var damage = source.Damage(target, 5f, tag: tag);
            Expect.Equal(1, watch.Seen.Count, "the damages the reaction saw");
            Expect.Same(tag, watch.Seen[0], "the Tag inside OnUnitDamaged");
            Expect.Same(tag, damage!.Tag, "the Tag of the returned damage");
        }

        /// <summary>A reaction that throws is reported, and the reactions after it still run.</summary>
        public static void ErrorIsolated(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            var dummy = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h000"), slot: 1))!;
            dummy.Add(new A());
            dummy.Add(new B());
            TestLog.Lines.Clear();
            Inbound.Damaged(dummy.Handle, null, 100f, true);
            ExpectLog(new string[] { "dummy damaged", "dummy comp damaged", "b damaged" }, "the log");
            Expect.Equal(1, world.Logged.Count, "the lines logged");
            Expect.Equal("Moonwell: A.OnDamaged failed: boom", world.Logged[0], "the error line");
        }
    }

    /// <summary>A game component that records the Tag of each damage it sees.</summary>
    public class TagWatch : GameComponent
    {
        /// <summary>The Tag of each damage, in order.</summary>
        public readonly System.Collections.Generic.List<object?> Seen = new System.Collections.Generic.List<object?>();

        /// <summary>Records the damage's Tag.</summary>
        public override void OnUnitDamaged(Damage damage)
        {
            Seen.Add(damage.Tag);
        }
    }

    /// <summary>A game component that records whether each damage it sees is an attack.</summary>
    public class AttackWatch : GameComponent
    {
        /// <summary>"attack" or "no attack" for each damage, in order.</summary>
        public readonly System.Collections.Generic.List<string> Seen = new System.Collections.Generic.List<string>();

        /// <summary>Records the damage.</summary>
        public override void OnUnitDamaged(Damage damage)
        {
            Seen.Add(damage.IsAttack ? "attack" : "no attack");
        }
    }
}
