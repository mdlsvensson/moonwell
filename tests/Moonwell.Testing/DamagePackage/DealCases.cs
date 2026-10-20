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
            var hit = source.Damage(target, 30f, DamageType.Fire, tag: "t");
            Expect.True(hit != null, "a hit is returned");
            Expect.True(hit!.IsCode, "the hit is code's");
            Expect.True((string?)hit.Tag == "t", "the hit has the tag");
            Expect.Equal((int)DamageType.Fire, (int)hit.DamageType, "the damage type");
            Expect.Near(30f, hit.Original, "the original amount");
            Expect.Near(70f, fake.Body(target.Handle).Life, "the target lost 30");
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
                    inner = target.Damage(source, 5f, DamageType.Normal);
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
                target.Damage(hit.Source!, 1f, DamageType.Normal);
            };
            other.Taken = hit =>
            {
                taken++;
                other.Damage(hit.Source!, 1f, DamageType.Normal);
            };
            fake.Hit(other.Handle, target.Handle, 1f, damageType: 4);
            Expect.Equal(8, taken, "eight hits in the chain");
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
                target.Damage(hit.Source!, 1f, DamageType.Normal);
            };
            other.Hurt = hit =>
            {
                hurt++;
                other.Damage(hit.Source!, 1f, DamageType.Normal);
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
}
