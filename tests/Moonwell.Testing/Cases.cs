using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Lists the cases, runs them, and prints the results.</summary>
    public static class Cases
    {
        /// <summary>Every case, grouped by file in task order.</summary>
        public static List<Case> All()
        {
            var list = new List<Case>();
            list.Add(new Case { Name = "harness.handles-are-keys", Run = HarnessCases.HandlesAreKeys });
            list.Add(new Case { Name = "harness.expect-fails", Run = HarnessCases.ExpectFails });
            list.Add(new Case { Name = "collections.set-keeps-order", Run = CollectionCases.SetKeepsOrder });
            list.Add(new Case { Name = "collections.set-remove-keeps-order", Run = CollectionCases.SetRemoveKeepsOrder });
            list.Add(new Case { Name = "collections.set-snapshot", Run = CollectionCases.SetSnapshot });
            list.Add(new Case { Name = "collections.map-keeps-order", Run = CollectionCases.MapKeepsOrder });
            list.Add(new Case { Name = "collections.map-remove", Run = CollectionCases.MapRemove });
            list.Add(new Case { Name = "errors.log-line", Run = ErrorCases.LogLine });
            list.Add(new Case { Name = "errors.screen-line", Run = ErrorCases.ScreenLine });
            list.Add(new Case { Name = "errors.shows-at", Run = ErrorCases.ShowsAt });
            list.Add(new Case { Name = "errors.report-throttles", Run = ErrorCases.ReportThrottles });
            list.Add(new Case { Name = "errors.report-many-keys", Run = ErrorCases.ReportManyKeys });
            list.Add(new Case { Name = "names.of", Run = ErrorCases.NamesOf });
            list.Add(new Case { Name = "world.advance", Run = ErrorCases.WorldAdvance });
            list.Add(new Case { Name = "ownership.ends-newest-first", Run = OwnershipCases.EndsNewestFirst });
            list.Add(new Case { Name = "ownership.ends-grandchildren", Run = OwnershipCases.EndsGrandchildren });
            list.Add(new Case { Name = "ownership.end-once", Run = OwnershipCases.EndOnce });
            list.Add(new Case { Name = "ownership.adopt-into-ended", Run = OwnershipCases.AdoptIntoEnded });
            list.Add(new Case { Name = "ownership.child-ends-alone", Run = OwnershipCases.ChildEndsAlone });
            list.Add(new Case { Name = "ownership.adopt-while-ending", Run = OwnershipCases.AdoptWhileEnding });
            list.Add(new Case { Name = "ownership.error-does-not-stop", Run = OwnershipCases.ErrorDoesNotStop });
            list.Add(new Case { Name = "ownership.run-sets-current", Run = OwnershipCases.RunSetsCurrent });
            list.Add(new Case { Name = "ownership.adopt-in-finish", Run = OwnershipCases.AdoptInFinish });
            list.Add(new Case { Name = "timers.after", Run = TimerCases.After });
            list.Add(new Case { Name = "timers.every", Run = TimerCases.Every });
            list.Add(new Case { Name = "timers.no-drift", Run = TimerCases.NoDrift });
            list.Add(new Case { Name = "timers.same-time-in-creation-order", Run = TimerCases.SameTimeInCreationOrder });
            list.Add(new Case { Name = "timers.end-cancels", Run = TimerCases.EndCancels });
            list.Add(new Case { Name = "timers.owned", Run = TimerCases.OwnerAdopts });
            list.Add(new Case { Name = "timers.current-in-callback", Run = TimerCases.CurrentInCallback });
            list.Add(new Case { Name = "timers.error-isolated", Run = TimerCases.ErrorIsolated });
            list.Add(new Case { Name = "timers.remaining", Run = TimerCases.Remaining });
            list.Add(new Case { Name = "hooks.overrides", Run = HookCases.Overrides });
            list.Add(new Case { Name = "hooks.cached", Run = HookCases.Cached });
            list.Add(new Case { Name = "events.need-listens-once", Run = HookCases.NeedListensOnce });
            return list;
        }

        /// <summary>Runs every case on a fresh world, and returns a PASS or FAIL line for each, then DONE.</summary>
        public static List<string> RunAll()
        {
            var lines = new List<string>();
            var cases = All();
            foreach (var testCase in cases)
            {
                var world = FakeWorld.Install();
                try
                {
                    testCase.Run(world);
                    lines.Add("PASS " + testCase.Name);
                }
                catch (Exception error)
                {
                    lines.Add("FAIL " + testCase.Name + ": " + error.Message);
                }
            }

            lines.Add("DONE " + cases.Count);
            return lines;
        }

        /// <summary>The map's entry point: prints every line of RunAll.</summary>
        public static void Main()
        {
            foreach (var line in RunAll())
            {
                Emit(line);
            }
        }

        /// <summary>Writes one line of output.</summary>
        // In Lua this is the print global, which the Lua test host captures.
        /// @CSharpLua.Template = "print({0})"
        public static void Emit(string line)
        {
            Console.WriteLine(line);
        }
    }
}
