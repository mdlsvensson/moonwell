using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for scopes: what code makes belongs to the scope named where it is made, and ends when that scope ends.</summary>
    public static class ScopeCases
    {
        /// <summary>A helper that makes a timer: it is told whose, and the timer is that scope's whichever reaction called it.</summary>
        public static Timer Smoke(Scope scope)
        {
            return scope.Every(1.0f, () => { });
        }

        /// <summary>A timer made with a scope's After is the scope's: it runs once, and one still waiting ends with the scope.</summary>
        public static void AfterBelongsToItsScope(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            var ran = 0;
            var first = scope.After(1.0f, () => { ran++; });
            Expect.Same(scope, first.Parent, "the timer's parent");
            world.Advance(1.05f);
            Expect.Equal(1, ran, "it ran once at 1 s");
            var second = scope.After(1.0f, () => { ran++; });
            scope.End();
            Expect.True(second.HasEnded, "the second ended with its scope");
            world.Advance(2.0f);
            Expect.Equal(1, ran, "and never ran");
        }

        /// <summary>A timer made with a scope's Every is the scope's: it runs at each interval until the scope ends.</summary>
        public static void EveryBelongsToItsScope(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            var ran = 0;
            var timer = scope.Every(1.0f, () => { ran++; });
            Expect.Same(scope, timer.Parent, "the timer's parent");
            world.Advance(2.05f);
            Expect.Equal(2, ran, "it ran twice in 2 s");
            scope.End();
            Expect.True(timer.HasEnded, "it ended with its scope");
            world.Advance(2.0f);
            Expect.Equal(2, ran, "and ran no more");
        }

        /// <summary>After in a unit's own OnDeath is the unit's, so it ends when the corpse goes; a helper given the game makes the game's.</summary>
        public static void DeathAfterIsTheUnits(FakeWorld world)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("n0TD"), () => new TimedDeath());
            var creep = (TimedDeath)Unit.Of(world.NewUnit(ObjectTypes.FourCC("n0TD"), slot: 24))!;
            creep.Kill();
            Expect.Same(creep, creep.Made!.Parent, "the timer OnDeath made with After is the unit's");
            Expect.Same(Game.Instance, creep.Smoked!.Parent, "the helper's timer is the scope it was given");
            world.Advance(5.0f);
            world.VanishUnit(creep.Handle);
            world.Advance(30.0f);
            Expect.True(creep.HasEnded, "the unit ended with its corpse");
            Expect.True(creep.Made!.HasEnded, "and the timer with it");
            Expect.Equal(0, creep.Ran, "which never ran");
        }

        /// <summary>The line a development build reports for a timer its scope ended in the instant it was made, before it ran.</summary>
        public const string NeverRan = "failed: ended with its scope in the instant it was made, before it ran; name a scope that lasts";

        /// <summary>How many timers that never ran were reported.</summary>
        public static int NeverRanReports(FakeWorld world)
        {
            var count = 0;
            foreach (var line in world.Logged)
            {
                if (line.Contains(NeverRan))
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Checks that a development build reported one timer that never ran, with the line, and a release build none.</summary>
        public static void ExpectNeverRanReported(FakeWorld world, string line)
        {
#if DEBUG
            Expect.Equal(1, NeverRanReports(world), "the reports of a timer that never ran: " + string.Join(" | ", world.Logged));
            Expect.True(world.Logged.Contains(line), "the line: " + string.Join(" | ", world.Logged));
#else
            Expect.Equal(0, NeverRanReports(world), "a release build reports none");
#endif
        }

        /// <summary>Checks that a development build reported a number of timers that never ran, and a release build none.</summary>
        public static void ExpectNeverRanCount(FakeWorld world, int count)
        {
#if DEBUG
            Expect.Equal(count, NeverRanReports(world), "the reports of a timer that never ran: " + string.Join(" | ", world.Logged));
#else
            Expect.Equal(0, NeverRanReports(world), "a release build reports none");
#endif
        }

        /// <summary>A timer a buff's OnExpire makes in the buff, which ends right after, is reported in a development build.</summary>
        public static void ExpireTimerIsReported(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            var buff = unit.Apply(new ExpiringTimerBuff());
            world.Advance(1.05f);
            Expect.True(buff.HasEnded, "the buff expired");
            Expect.True(buff.Made!.HasEnded, "the timer OnExpire made ended with it");
            ExpectNeverRanReported(world, "Moonwell: ExpiringTimerBuff.Timer " + NeverRan);
        }

        /// <summary>A timer whose scope ends in the instant it was made is reported in a development build.</summary>
        public static void TimerEndedAsMadeIsReported(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            scope.After(1.0f, () => { });
            scope.End();
            ExpectNeverRanReported(world, "Moonwell: TestScope.Timer " + NeverRan);
        }

        /// <summary>A timer made in a scope that has ended ends at once and is reported in a development build.</summary>
        public static void TimerInEndedScopeIsReported(FakeWorld world)
        {
            var ended = new TestScope("ended", new List<string>());
            ended.End();
            var timer = ended.After(1.0f, () => { });
            Expect.True(timer.HasEnded, "the timer ended at once");
            ExpectNeverRanReported(world, "Moonwell: TestScope.Timer " + NeverRan);
        }

        /// <summary>A timer a scope makes in itself while it ends is reported in a development build.</summary>
        public static void TimerMadeWhileEndingIsReported(FakeWorld world)
        {
            var scope = new EndingTimerScope();
            scope.End();
            Expect.True(scope.Made!.HasEnded, "the timer ended with the scope");
            ExpectNeverRanReported(world, "Moonwell: EndingTimerScope.Timer " + NeverRan);
        }

        /// <summary>A timer ended by its own End in the instant it was made is not reported.</summary>
        public static void TimerEndedByItself(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            var timer = scope.After(1.0f, () => { });
            timer.End();
            Expect.Equal(0, NeverRanReports(world), "the reports");
        }

        /// <summary>A timer whose scope ends at a later instant, before the timer ran, is not reported.</summary>
        public static void TimerEndedLater(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            scope.After(1.0f, () => { });
            world.Advance(0.5f);
            scope.End();
            Expect.Equal(0, NeverRanReports(world), "the reports");
        }

        /// <summary>A timer that ran and then ended its scope in the instant it was made is not reported.</summary>
        public static void TimerThatRan(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            var ran = false;
            Game.Instance.After(1.0f, () =>
            {
                scope.After(0.0f, () =>
                {
                    ran = true;
                    scope.End();
                });
            });
            world.Advance(1.05f);
            Expect.True(ran, "the timer made at 1 s ran in that instant");
            Expect.Equal(0, NeverRanReports(world), "the reports");
        }

        /// <summary>Every of 0 is refused and reported once, as a refusal; the timer given back has no parent and is not also reported as never run.</summary>
        public static void RefusedEveryOnce(FakeWorld world)
        {
            var scope = new TestScope("scope", new List<string>());
            var timer = scope.Every(0.0f, () => { });
            Expect.True(timer.Parent == null, "the refused timer has no parent");
            Expect.Equal(1, world.Logged.Count, "one line: " + string.Join(" | ", world.Logged));
            Expect.True(world.Logged[0].StartsWith("Moonwell: TestScope.Every failed: seconds must be more than 0", System.StringComparison.Ordinal), "the refusal");
        }

        /// <summary>A helper's timers are the scope it is given each time.</summary>
        public static void HelperUsesItsScope(FakeWorld world)
        {
            var a = new TestScope("a", new List<string>());
            var b = new TestScope("b", new List<string>());
            Expect.Same(a, Smoke(a).Parent, "the timer made for a");
            Expect.Same(b, Smoke(b).Parent, "the timer made for b");
        }
    }
}
