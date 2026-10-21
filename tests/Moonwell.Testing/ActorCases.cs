using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check actors and their components, the game's reactions to start, and the players.</summary>
    public static class ActorCases
    {
        /// <summary>A component that logs its name when it is removed.</summary>
        private class Named : GameComponent
        {
            private readonly string name;
            private readonly List<string> log;

            /// <summary>Makes a component with a name that logs the name when it is removed.</summary>
            public Named(string name, List<string> log)
            {
                this.name = name;
                this.log = log;
            }

            /// <summary>Logs the name.</summary>
            protected override void OnRemoved()
            {
                log.Add(name);
            }
        }

        /// <summary>A named component of the class A.</summary>
        private sealed class A : Named
        {
            /// <summary>Makes an A that logs its name when it is removed.</summary>
            public A(List<string> log)
                : base("A", log)
            {
            }
        }

        /// <summary>A named component of the class B.</summary>
        private sealed class B : Named
        {
            /// <summary>Makes a B that logs its name when it is removed.</summary>
            public B(List<string> log)
                : base("B", log)
            {
            }
        }

        /// <summary>A component that counts its adds and keeps the current owner it saw in OnAdded.</summary>
        private sealed class Probe : GameComponent
        {
            /// <summary>How many times OnAdded ran.</summary>
            public int Adds;

            /// <summary>The current owner seen inside OnAdded.</summary>
            public Owner? Seen;

            /// <summary>Counts the add and keeps the current owner.</summary>
            protected override void OnAdded()
            {
                Adds++;
                Seen = Owner.Current;
            }
        }

        /// <summary>A component that starts a repeating timer when it is added.</summary>
        private sealed class Ticker : GameComponent
        {
            /// <summary>How many times the timer ran.</summary>
            public int Runs;

            /// <summary>Starts a timer every half second, which the component owns.</summary>
            protected override void OnAdded()
            {
                Timer.Every(0.5f, () => { Runs++; });
            }
        }

        /// <summary>A game component that logs its name at the game's start.</summary>
        private sealed class Starter : GameComponent
        {
            private readonly string name;
            private readonly List<string> log;

            /// <summary>Makes a game component that logs its name at the game's start.</summary>
            public Starter(string name, List<string> log)
            {
                this.name = name;
                this.log = log;
            }

            /// <summary>Logs the name.</summary>
            public override void OnGameStart()
            {
                log.Add(name);
            }
        }

        /// <summary>A game component that removes a victim and adds a new Starter named d each time the game starts.</summary>
        private sealed class Remover : GameComponent
        {
            private readonly string name;
            private readonly List<string> log;
            private readonly GameComponent victim;

            /// <summary>Makes a component that logs its name, removes the victim unless it has ended, and adds a d.</summary>
            public Remover(string name, List<string> log, GameComponent victim)
            {
                this.name = name;
                this.log = log;
                this.victim = victim;
            }

            /// <summary>Logs the name, removes the victim while it has not ended, and adds a new d.</summary>
            public override void OnGameStart()
            {
                log.Add(name);
                if (!victim.HasEnded)
                {
                    Game.Remove(victim);
                }

                Game.Add(new Starter("d", log));
            }
        }

        /// <summary>A player component that logs its name at the game's start.</summary>
        private sealed class PlayerStarter : PlayerComponent
        {
            private readonly string name;
            private readonly List<string> log;

            /// <summary>Makes a player component that logs its name at the game's start.</summary>
            public PlayerStarter(string name, List<string> log)
            {
                this.name = name;
                this.log = log;
            }

            /// <summary>Logs the name.</summary>
            public override void OnGameStart()
            {
                log.Add(name);
            }
        }

        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(List<string> log, string[] expected, string what)
        {
            Expect.Equal(expected.Length, log.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], log[i], what + ": line " + i);
            }
        }

        /// <summary>Get gives the first component of a class in add order; GetAll and ComponentCount count them all.</summary>
        public static void AddAndGet(FakeWorld world)
        {
            var log = new List<string>();
            var actor = new TestActor();
            var first = actor.Add(new A(log));
            actor.Add(new B(log));
            actor.Add(new A(log));
            Expect.Same(first, actor.Get<A>(), "the first A");
            Expect.Equal(2, actor.GetAll<A>().Count, "the A components");
            Expect.Equal(3, actor.ComponentCount, "the components");
            Expect.True(actor.Has<B>(), "the actor has a B");
            Expect.True(!actor.Has<Probe>(), "the actor has no Probe");
        }

        /// <summary>Remove ends the component, which logs its removal and leaves the actor's list.</summary>
        public static void RemoveOne(FakeWorld world)
        {
            var log = new List<string>();
            var actor = new TestActor();
            var first = actor.Add(new A(log));
            var second = actor.Add(new A(log));
            actor.Remove(first);
            ExpectLog(log, new string[] { "A" }, "log after the first A is removed");
            Expect.True(first.HasEnded, "the removed A has ended");
            Expect.Same(second, actor.Get<A>(), "the first A is gone");
        }

        /// <summary>RemoveAll removes every component of a class and returns how many.</summary>
        public static void RemoveAllOf(FakeWorld world)
        {
            var log = new List<string>();
            var actor = new TestActor();
            actor.Add(new A(log));
            actor.Add(new B(log));
            actor.Add(new A(log));
            Expect.Equal(2, actor.RemoveAll<A>(), "the A components removed");
            Expect.Equal(1, actor.ComponentCount, "the components left");
        }

        /// <summary>Remove of a class removes the first component of that class, and says whether there was one.</summary>
        public static void RemoveOf(FakeWorld world)
        {
            var log = new List<string>();
            var actor = new TestActor();
            var first = actor.Add(new A(log));
            actor.Add(new B(log));
            var second = actor.Add(new A(log));
            Expect.True(actor.Remove<A>(), "an A was removed");
            Expect.True(first.HasEnded, "the first A has ended");
            Expect.Same(second, actor.Get<A>(), "the second A is left");
            Expect.Equal(2, actor.ComponentCount, "the components left");
            actor.Remove<A>();
            Expect.True(!actor.Remove<A>(), "no A is left to remove");
        }

        /// <summary>Inside OnAdded, Owner.Current is the component.</summary>
        public static void CurrentInOnAdded(FakeWorld world)
        {
            var actor = new TestActor();
            var probe = actor.Add(new Probe());
            Expect.Equal(1, probe.Adds, "OnAdded ran once");
            Expect.Same(probe, probe.Seen, "Owner.Current inside OnAdded");
        }

        /// <summary>When the actor ends, its components end newest first, and each logs its removal.</summary>
        public static void EndRemovesLastFirst(FakeWorld world)
        {
            var log = new List<string>();
            var actor = new TestActor();
            actor.Add(new Named("x", log));
            actor.Add(new Named("y", log));
            actor.Add(new Named("z", log));
            actor.End();
            ExpectLog(log, new string[] { "z", "y", "x" }, "log after the actor ends");
        }

        /// <summary>A component added to an actor that has ended ends at once, and its OnAdded does not run.</summary>
        public static void AddToEnded(FakeWorld world)
        {
            var actor = new TestActor();
            actor.End();
            var probe = actor.Add(new Probe());
            Expect.True(probe.HasEnded, "the component has ended");
            Expect.Equal(0, probe.Adds, "OnAdded runs for it");
            Expect.Equal(0, actor.ComponentCount, "components on the ended actor");
        }

        /// <summary>A timer a component starts in OnAdded belongs to it, and ends when the component is removed.</summary>
        public static void ComponentOwnsTimer(FakeWorld world)
        {
            var actor = new TestActor();
            var ticker = actor.Add(new Ticker());
            world.Advance(1.1f);
            Expect.Equal(2, ticker.Runs, "runs after 1.1 seconds");
            actor.Remove(ticker);
            world.Advance(2f);
            Expect.Equal(2, ticker.Runs, "runs after the component was removed");
        }

        /// <summary>An error in OnAdded is reported, and the component stays on the actor.</summary>
        public static void ErrorInOnAdded(FakeWorld world)
        {
            var actor = new TestActor();
            var thrower = actor.Add(new Thrower());
            Expect.Equal(1, world.Logged.Count, "logged lines");
            Expect.Equal("Moonwell: Thrower.OnAdded failed: boom", world.Logged[0], "the logged line");
            Expect.Equal(1, actor.ComponentCount, "components after OnAdded throws");
            Expect.True(!thrower.HasEnded, "the component stays");
        }

        /// <summary>At the game's start, the game's components run first, then each existing player's, in slot order.</summary>
        public static void GameStartOrder(FakeWorld world)
        {
            var log = new List<string>();
            Game.Instance.Add(new Starter("game", log));
            Player.Get(2).Add(new PlayerStarter("p2", log));
            Player.Get(0).Add(new PlayerStarter("p0", log));
            Inbound.GameStarted();
            ExpectLog(log, new string[] { "game", "p0", "p2" }, "the order of OnGameStart");
        }

        /// <summary>A component removed earlier in the game's start does not run; one added during it runs at the next start.</summary>
        public static void GameDispatchRemoval(FakeWorld world)
        {
            var log = new List<string>();
            var c = new Starter("c", log);
            Game.Instance.Add(new Remover("a", log, c));
            Game.Instance.Add(new Starter("b", log));
            Game.Instance.Add(c);
            Inbound.GameStarted();
            ExpectLog(log, new string[] { "a", "b" }, "log of the first game start");
            log.Clear();
            Inbound.GameStarted();
            ExpectLog(log, new string[] { "a", "b", "d" }, "log of the second game start");
        }

        /// <summary>Player.Get makes one player per slot, and the handle's player is the same object.</summary>
        public static void PlayersGet(FakeWorld world)
        {
            var three = Player.Get(3);
            Expect.Equal(3, three.Slot, "the slot");
            Expect.Same(three, Player.Get(3), "the player asked for again");
            Expect.Same(three, Player.Of(world.PlayerAt(3)), "the player of its handle");
        }

        /// <summary>The named players are the players of slots 0 to 23.</summary>
        public static void PlayersNamed(FakeWorld world)
        {
            Expect.Same(Player.Get(0), Player.One, "Player.One");
            Expect.Same(Player.Get(1), Player.Two, "Player.Two");
            Expect.Same(Player.Get(11), Player.Twelve, "Player.Twelve");
            Expect.Same(Player.Get(23), Player.TwentyFour, "Player.TwentyFour");
        }

        /// <summary>The neutral players are the players of the slots the game gives them.</summary>
        public static void PlayersNeutral(FakeWorld world)
        {
            world.NeutralAggressive = 24;
            world.NeutralVictim = 25;
            world.NeutralExtra = 26;
            world.NeutralPassive = 27;
            Expect.Same(Player.Get(24), Player.NeutralAggressive, "Player.NeutralAggressive");
            Expect.Same(Player.Get(25), Player.NeutralVictim, "Player.NeutralVictim");
            Expect.Same(Player.Get(26), Player.NeutralExtra, "Player.NeutralExtra");
            Expect.Same(Player.Get(27), Player.NeutralPassive, "Player.NeutralPassive");
        }

        /// <summary>A player's start location is the game's for its slot.</summary>
        public static void PlayersStart(FakeWorld world)
        {
            world.SetStart(2, -512f, 768f);
            Expect.True(Player.Three.StartX == -512f, "StartX");
            Expect.True(Player.Three.StartY == 768f, "StartY");
        }

        /// <summary>A slot outside the player slots has no player object, and asking for it does not fail.</summary>
        public static void ExistingOutOfRange(FakeWorld world)
        {
            Expect.True(Player.Existing(-1) == null, "a slot below the range");
            Expect.True(Player.Existing(Player.MaxSlots) == null, "a slot above the range");
        }
    }
}
