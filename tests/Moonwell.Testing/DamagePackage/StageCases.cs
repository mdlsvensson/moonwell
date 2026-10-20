namespace Moonwell.Testing
{
    /// <summary>Cases that check the damage package's before-armor and after-armor stages and their order.</summary>
    public static class StageCases
    {
        /// <summary>The fake damage world of the current case.</summary>
        private static FakeDamageWorld fake = null!;

        /// <summary>The fake damage world of the current case, for the other damage package cases.</summary>
        internal static FakeDamageWorld Fake => fake;

        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        internal static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length && i < TestLog.Lines.Count; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>Installs the package on a fake, registers SourceUnit (H001) and TargetUnit (h001), and makes one of each: the source for player 0, the target for player 1.</summary>
        internal static void Arena(FakeWorld world, out SourceUnit source, out TargetUnit target)
        {
            fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("H001"), () => new SourceUnit());
            ObjectTypes.Unit(ObjectTypes.FourCC("h001"), () => new TargetUnit());
            source = (SourceUnit)Unit.Of(world.NewUnit(ObjectTypes.FourCC("H001"), slot: 0))!;
            target = (TargetUnit)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h001"), slot: 1))!;
            TestLog.Lines.Clear();
        }

        /// <summary>Each stage runs the source and its components, the target and its components, the game's, the target's player's and the source's player's; after armor core's reactions come first.</summary>
        public static void Order(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            Game.Instance.Add(new GameHitWatch());
            Player.Get(0).Add(new PlayerHitWatch("p0"));
            Player.Get(1).Add(new PlayerHitWatch("p1"));
            fake.Hit(source.Handle, target.Handle, 100f, isAttack: true);
            ExpectLog(new string[]
            {
                "source dealing", "source comp dealing", "target taking", "target comp taking", "game before", "p1 before", "p0 before",
                "source core dealt", "target core damaged",
                "source dealt", "source comp dealt", "target taken", "target comp taken", "game after", "p1 after", "p0 after",
            }, "the log");
        }

        /// <summary>An amount and a damage type set before armor reach the game.</summary>
        public static void BeforeArmorChanges(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            fake.Body(target.Handle).Life = 1000f;
            var seenAmount = 0f;
            var seenType = DamageKind.Unknown;
            target.Taking = hit =>
            {
                hit.Amount = hit.Amount * 2f;
                hit.DamageKind = DamageKind.Fire;
            };
            target.Taken = hit =>
            {
                seenAmount = hit.Amount;
                seenType = hit.DamageKind;
            };
            fake.Hit(source.Handle, target.Handle, 100f, damageType: 4);
            Expect.Near(200f, seenAmount, "the amount after armor");
            Expect.Equal((int)DamageKind.Fire, (int)seenType, "the damage type after armor");
            Expect.Near(800f, fake.Body(target.Handle).Life, "the life after the doubled hit");
        }

        /// <summary>Armor set before armor counts for the hit, and the target has its own armor back after; a bonus on the armor is kept.</summary>
        public static void ArmorForOneHit(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            var body = fake.Body(target.Handle);
            body.Life = 1000f;
            body.Armor = 2f;
            body.ArmorBonus = 3f;
            var during = 0f;
            target.Taking = hit => hit.Armor = 0f;
            fake.Hit(source.Handle, target.Handle, 100f, damageType: 4, inside: () => during = fake.Armor(target.Handle));
            Expect.Near(0f, during, "the armor during the hit");
            Expect.Near(900f, body.Life, "the hit was not reduced by armor");
            Expect.Near(2f, body.Armor, "the base armor after the hit");
            Expect.Near(5f, fake.Armor(target.Handle), "the armor after the hit");
        }

        /// <summary>An amount set after armor is what the target loses.</summary>
        public static void AfterArmorChanges(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            target.Taken = hit => hit.Amount = hit.Amount / 2f;
            fake.Hit(source.Handle, target.Handle, 40f, damageType: 4);
            Expect.Near(80f, fake.Body(target.Handle).Life, "the life after the halved hit");
        }

        /// <summary>A type set after armor is reported in a development build, since the game would ignore it, and the reactions after it still run.</summary>
        public static void TypeSetAfterArmorReported(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            target.Taken = hit => hit.DamageKind = DamageKind.Fire;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
#if DEBUG
            Expect.Equal(1, world.Logged.Count, "one error is logged in a development build");
#else
            Expect.Equal(0, world.Logged.Count, "a release build leaves the check out");
#endif
            Expect.True(TestLog.Lines.Contains("target comp taken"), "the component's reaction still ran");
        }

        /// <summary>A reaction that removes its unit stops the unit's later reactions; the map's still run.</summary>
        public static void RemovedMidStage(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            Game.Instance.Add(new GameHitWatch());
            ObjectTypes.Unit(ObjectTypes.FourCC("h002"), () => new Unit());
            var plain = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h002"), slot: 1))!;
            plain.Add(new RemoverBeforeArmor());
            plain.Add(new TargetComp());
            TestLog.Lines.Clear();
            fake.Hit(source.Handle, plain.Handle, 10f);
            ExpectLog(new string[] { "source dealing", "source comp dealing", "game before", "source core dealt", "source dealt", "source comp dealt", "game after" }, "the log");
            Expect.Equal(0, world.Logged.Count, "nothing is reported");
        }

        /// <summary>A unit that is both melee and ranged deals a ranged hit when the hit has no sound, a melee one when it has.</summary>
        public static void MeleeOrRanged(FakeWorld world)
        {
            Arena(world, out var source, out var target);
            fake.Body(source.Handle).Melee = true;
            fake.Body(source.Handle).Ranged = true;
            var melee = false;
            var ranged = false;
            target.Taking = hit =>
            {
                melee = hit.IsMelee;
                ranged = hit.IsRanged;
            };
            fake.Hit(source.Handle, target.Handle, 10f, attackType: 2, damageType: 4, weapon: 0, isAttack: true);
            Expect.True(ranged && !melee, "a hit with no sound is ranged");
            fake.Hit(source.Handle, target.Handle, 10f, attackType: 1, damageType: 4, weapon: 5, isAttack: true);
            Expect.True(melee && !ranged, "a hit with a sound is melee");
        }
    }
}
