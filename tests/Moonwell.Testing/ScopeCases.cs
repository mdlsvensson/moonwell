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
