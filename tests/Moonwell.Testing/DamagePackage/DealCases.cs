namespace Moonwell.Testing
{
    /// <summary>Cases that check damage dealt from code, and chains of hits dealt from reactions.</summary>
    public static class DealCases
    {
        /// <summary>Makes another TargetUnit for a slot.</summary>
        private static TargetUnit Another(FakeWorld world, int slot)
        {
            return (TargetUnit)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h001"), slot: slot))!;
        }

        /// <summary>Unit.Damage returns the hit, marked as code with its tag and type, after the target lost the amount.</summary>
        public static void DealReturnsHit(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var hit = source.Damage(target, 30f, DamageKind.Fire, tag: "t");
            Expect.True(hit != null, "a hit is returned");
            Expect.True(hit!.IsCode, "the hit is code's");
            Expect.True((string?)hit.Tag == "t", "the hit has the tag");
            Expect.Equal((int)DamageKind.Fire, (int)hit.DamageKind, "the damage type");
            Expect.Near(30f, hit.Original, "the original amount");
            Expect.Near(70f, fake.Body(target.Handle).Life, "the target lost 30");
        }

        /// <summary>The attack type given to Unit.Damage reaches the game in its number, and a hit with none is a spell's.</summary>
        public static void DealAttackType(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var during = -1;
            target.Taking = hit => during = fake.EventAttackType();
            var siege = source.Damage(target, 10f, attackType: AttackType.Siege);
            Expect.Equal(3, during, "the game's attack type of the siege hit");
            Expect.Equal((int)AttackType.Siege, (int)siege!.AttackType, "the hit's attack type");
            Expect.True(!siege.IsSpell, "a siege hit is not a spell's");
            var plain = source.Damage(target, 10f);
            Expect.Equal(0, during, "the game's attack type of a hit with none");
            Expect.Equal((int)AttackType.Spells, (int)plain!.AttackType, "the hit's default attack type");
            Expect.True(plain.IsSpell, "a hit with none is a spell's");
        }

        /// <summary>With no reaction anywhere, the first Unit.Damage makes the package listen, so the hit is filled in.</summary>
        public static void DealStartsListening(FakeWorld world)
        {
            var fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("h002"), () => new Unit());
            var first = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h002"), slot: 0))!;
            var second = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h002"), slot: 1))!;
            Expect.True(!fake.Listening, "not listening before");
            var hit = first.Damage(second, 10f);
            Expect.True(fake.Listening, "listening after");
            Expect.True(hit != null, "a hit is returned");
        }

        /// <summary>Damage dealt before armor runs whole inside the outer hit, before the outer's after armor.</summary>
        public static void DealInsideReactionNests(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            Hit? inner = null;
            var sourceLifeInTaken = 0f;
            target.Taking = hit =>
            {
                if (inner == null)
                {
                    inner = target.Damage(source, 5f, DamageKind.Normal);
                }
            };
            target.Taken = hit => sourceLifeInTaken = fake.Body(source.Handle).Life;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            Expect.True(inner != null && inner.IsCode, "the inner hit was dealt");
            Expect.Near(95f, sourceLifeInTaken, "the inner hit landed before the outer's after armor");
        }

        /// <summary>Two units that hit back after armor make a chain of 8 hits, then the next is refused and reported once.</summary>
        public static void ChainCut(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var other = Another(world, 0);
            var taken = 0;
            target.Taken = hit =>
            {
                taken++;
                target.Damage(hit.Source!, 1f, DamageKind.Normal);
            };
            other.Taken = hit =>
            {
                taken++;
                other.Damage(hit.Source!, 1f, DamageKind.Normal);
            };
            fake.Hit(other.Handle, target.Handle, 1f, damageType: 4);
            Expect.Equal(8, taken, "eight hits in the chain");
            Expect.Equal(1, world.Logged.Count, "one error is reported");
        }

        /// <summary>Hitting back from core's OnDamaged counts too: with the package, the chain stops at 8.</summary>
        public static void ChainThroughOnDamaged(FakeWorld world)
        {
            var fake = new FakeDamageWorld();
            DamageSystem.Install(fake);
            ObjectTypes.Unit(ObjectTypes.FourCC("h003"), () => new CoreHitback());
            var first = (CoreHitback)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h003"), slot: 0))!;
            var second = (CoreHitback)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h003"), slot: 1))!;
            CoreHitback.Count = 0;
            fake.Hit(first.Handle, second.Handle, 1f, damageType: 4);
            Expect.Equal(8, CoreHitback.Count, "eight hits in the chain");
            Expect.Equal(1, world.Logged.Count, "one error is reported");
        }

        /// <summary>The same with OnHurt, across 0 s timers: the chain still stops at 8.</summary>
        public static void ChainAcrossFrames(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var other = Another(world, 0);
            var hurt = 0;
            target.Hurt = hit =>
            {
                hurt++;
                target.Damage(hit.Source!, 1f, DamageKind.Normal);
            };
            other.Hurt = hit =>
            {
                hurt++;
                other.Damage(hit.Source!, 1f, DamageKind.Normal);
            };
            fake.Hit(other.Handle, target.Handle, 1f, damageType: 4);
            var rounds = 0;
            while (fake.RunZero() && rounds < 20)
            {
                rounds++;
            }

            Expect.Equal(8, hurt, "eight hits landed");
            Expect.Equal(1, world.Logged.Count, "one error is reported");
        }
    }

    /// <summary>A unit whose core OnDamaged hits the source back for 1, counting every hit it takes.</summary>
    public class CoreHitback : Unit
    {
        /// <summary>The hits every CoreHitback has taken.</summary>
        public static int Count;

        /// <summary>Counts the hit and hits back.</summary>
        public override void OnDamaged(Damage damage)
        {
            Count++;
            if (damage.Source != null)
            {
                this.Damage(damage.Source, 1f, DamageKind.Normal);
            }
        }
    }
}
