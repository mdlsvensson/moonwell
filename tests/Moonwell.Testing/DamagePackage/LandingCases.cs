namespace Moonwell.Testing
{
    /// <summary>Cases that check the lethal, landed and burst stages, spell immunity, and hits inside hits.</summary>
    public static class LandingCases
    {
        /// <summary>Makes another TargetUnit for player 1.</summary>
        private static TargetUnit Another(FakeWorld world)
        {
            return (TargetUnit)Unit.Of(world.NewUnit(ObjectTypes.FourCC("h001"), slot: 1))!;
        }

        /// <summary>Makes a Bonuser (H00A) for player 0, whose hits add 20 from code after armor.</summary>
        private static Unit Bonus(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("H00A"), () => new Bonuser());
            return Unit.Of(world.NewUnit(ObjectTypes.FourCC("H00A"), slot: 0))!;
        }

        /// <summary>A bonus hit dealt after armor lowers the life before the outer hit's lethal check: a 95 hit after a 20 bonus on 100 life is lethal, and Spare keeps the target at 1.</summary>
        public static void LethalAfterBonus(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var bonuser = Bonus(world);
            var lethal = 0;
            target.Lethal = hit =>
            {
                lethal++;
                hit.Spare();
            };
            fake.Hit(bonuser.Handle, target.Handle, 95f, damageType: 4);
            Expect.Equal(1, lethal, "the lethal reaction ran for the hit that would kill");
            Expect.Near(1f, fake.Body(target.Handle).Life, "the spared target keeps 1 life");
        }

        /// <summary>Two hits on one target in one moment, the second killing: only the second says Killed.</summary>
        public static void KilledOnlyKillingHit(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Life = 50f;
            var killed = new System.Collections.Generic.List<bool>();
            target.Hurt = hit => killed.Add(hit.Killed);
            fake.Hit(source.Handle, target.Handle, 30f, damageType: 4);
            fake.Hit(source.Handle, target.Handle, 30f, damageType: 4);
            fake.RunZero();
            Expect.Equal(2, killed.Count, "two hits landed");
            Expect.True(!killed[0], "the first hit, which left 20 life, did not kill");
            Expect.True(killed[1], "the second hit killed");
        }

        /// <summary>A bonus hit that kills before the outer hit lands gets the kill: the outer hit runs no lethal stage and does not say Killed.</summary>
        public static void BonusKillCreditsTheBonus(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var bonuser = Bonus(world);
            fake.Body(target.Handle).Life = 15f;
            var lethal = new System.Collections.Generic.List<string>();
            var landed = new System.Collections.Generic.List<string>();
            target.Lethal = hit => lethal.Add(hit.IsCode ? "bonus" : "outer");
            target.Hurt = hit => landed.Add((hit.IsCode ? "bonus " : "outer ") + (hit.Killed ? "killed" : "alive"));
            fake.Hit(bonuser.Handle, target.Handle, 10f, damageType: 4);
            fake.RunZero();
            Expect.Equal("bonus", string.Join("|", lethal), "only the bonus hit's lethal stage ran");
            Expect.Equal("bonus killed|outer alive", string.Join("|", landed), "the bonus hit got the kill");
        }

        /// <summary>Once a lethal reaction spares the target, the later lethal reactions do not run: the hit no longer kills.</summary>
        public static void LethalStopsWhenSpared(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var watch = new StageWatch();
            Game.Instance.Add(watch);
            fake.Body(target.Handle).Life = 50f;
            target.Lethal = hit => hit.Spare();
            fake.Hit(source.Handle, target.Handle, 100f, damageType: 4);
            Expect.Equal("", string.Join("|", watch.Seen), "the game's lethal reaction did not run");
            Expect.Near(1f, fake.Body(target.Handle).Life, "the target was spared");
        }

        /// <summary>A target removed by a reaction before the 0 s timer is not killed by the hit, and nothing throws.</summary>
        public static void RemovedTargetNotKilled(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var watch = new StageWatch();
            Game.Instance.Add(watch);
            target.Taken = hit => target.Remove();
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            fake.RunZero();
            Expect.Equal("landed alive", string.Join("|", watch.Seen), "the hit landed without killing");
            Expect.Equal(0, world.Logged.Count, "nothing was reported");
        }

        /// <summary>A lethal hit runs OnLethal; Spare keeps the target at 1 life; without it the target dies, and OnHurt says so.</summary>
        public static void LethalSpare(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Life = 50f;
            var killed = true;
            target.Lethal = hit => hit.Spare();
            target.Hurt = hit => killed = hit.Killed;
            fake.Hit(source.Handle, target.Handle, 100f, damageType: 4);
            fake.RunZero();
            Expect.Near(1f, fake.Body(target.Handle).Life, "the spared target's life");
            Expect.True(!killed, "the spared target was not killed");

            var other = Another(world);
            fake.Body(other.Handle).Life = 50f;
            other.Hurt = hit => killed = hit.Killed;
            fake.Hit(source.Handle, other.Handle, 100f, damageType: 4);
            fake.RunZero();
            Expect.True(killed, "the other target was killed");
        }

        /// <summary>A hit that leaves the target alive runs no lethal reaction.</summary>
        public static void LethalOnlyWhenLethal(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Life = 50f;
            var lethal = false;
            target.Lethal = hit => lethal = true;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            Expect.True(!lethal, "no lethal reaction");
        }

        /// <summary>OnHurt runs after the 0 s timer, once.</summary>
        public static void LandedAfterZero(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var hurt = 0;
            target.Hurt = hit => hurt++;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            Expect.Equal(0, hurt, "not hurt before the timer");
            fake.RunZero();
            Expect.Equal(1, hurt, "hurt once after the timer");
        }

        /// <summary>Every hit of one source before the 0 s timer is one burst, with the number of units it hit.</summary>
        public static void BurstCounts(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var second = Another(world);
            var third = Another(world);
            var bursts = 0;
            var hits = 0;
            var targets = 0;
            source.Bursted = burst =>
            {
                bursts++;
                hits = burst.Hits.Count;
                targets = burst.Targets;
            };
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            fake.Hit(source.Handle, second.Handle, 10f, damageType: 4);
            fake.Hit(source.Handle, third.Handle, 10f, damageType: 4);
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            fake.RunZero();
            Expect.Equal(1, bursts, "one burst");
            Expect.Equal(4, hits, "four hits");
            Expect.Equal(3, targets, "three targets");
        }

        /// <summary>Magic on a spell-immune unit runs the before-armor stage only: no after armor, no landing, no burst.</summary>
        public static void ImmunityStops(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Immune = true;
            var hurt = false;
            var bursts = 0;
            target.Hurt = hit => hurt = true;
            source.Bursted = burst => bursts++;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 14);
            fake.RunZero();
            Expect.True(TestLog.Lines.Contains("target taking"), "the before-armor stage ran");
            Expect.True(!TestLog.Lines.Contains("target taken"), "no after-armor stage");
            Expect.True(!hurt, "no landing");
            Expect.Equal(0, bursts, "no burst");
        }

        /// <summary>A damage type changed before armor decides whether spell immunity stops the hit.</summary>
        public static void ImmunityAfterTypeChange(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Immune = true;
            target.Taking = hit => hit.DamageKind = DamageKind.Normal;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 14);
            Expect.True(TestLog.Lines.Contains("target taken"), "the after-armor stage ran");
        }

        /// <summary>A hit spell immunity stops, inside another, is not left open: the outer hit still finds itself after armor and has its armor back by then.</summary>
        public static void ImmunityInsideHit(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var immune = Another(world);
            fake.Body(immune.Handle).Immune = true;
            fake.Body(target.Handle).Armor = 2f;
            var armorInTaken = -1f;
            target.Taking = hit => hit.Armor = 10f;
            target.Taken = hit => armorInTaken = fake.Armor(target.Handle);
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4, inside: () => fake.Hit(source.Handle, immune.Handle, 5f, damageType: 14));
            Expect.Near(2f, armorInTaken, "the outer target's armor is back when its after-armor stage runs");
        }

        /// <summary>A hit inside another runs its stages first; the outer keeps its own amount and puts its armor change back.</summary>
        public static void NestedKeepsOuter(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            var linked = Another(world);
            fake.Body(target.Handle).Life = 1000f;
            fake.Body(target.Handle).Armor = 2f;
            target.Taking = hit => hit.Armor = 10f;
            target.Taken = hit => TestLog.Lines.Add("outer " + (int)(hit.Amount * 10f + 0.5f));
            linked.Taken = hit => TestLog.Lines.Add("inner " + (int)(hit.Amount * 10f + 0.5f));
            TestLog.Lines.Clear();
            fake.Hit(source.Handle, target.Handle, 100f, damageType: 4, inside: () => fake.Hit(source.Handle, linked.Handle, 20f, damageType: 24));
            var inner = TestLog.Lines.IndexOf("inner 200");
            var outer = TestLog.Lines.IndexOf("outer 625");
            Expect.True(inner >= 0 && outer > inner, "the inner hit's after armor comes first, the outer keeps 62.5");
            Expect.Near(2f, fake.Armor(target.Handle), "the outer target's armor is back");
        }

        /// <summary>A hit whose after-armor event never comes is ended at the 0 s timer: its armor goes back and it does not land.</summary>
        public static void StrayOpenEnded(FakeWorld world)
        {
            StageCases.Arena(world, out var source, out var target);
            var fake = StageCases.Fake;
            fake.Body(target.Handle).Armor = 2f;
            var hurt = false;
            target.Taking = hit => hit.Armor = 0f;
            target.Hurt = hit => hurt = true;
            fake.WithholdNextDamaged = true;
            fake.Hit(source.Handle, target.Handle, 10f, damageType: 4);
            fake.RunZero();
            Expect.Near(2f, fake.Armor(target.Handle), "the armor is back");
            Expect.True(!hurt, "the hit did not land");
        }
    }
}
