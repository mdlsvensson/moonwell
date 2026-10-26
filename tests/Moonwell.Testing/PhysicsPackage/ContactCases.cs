using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for a missile's contacts with units, destructables and items: the swept test, CanHit, the order of the reactions, damage, the outcomes Stop, Bounce and Pass, shoves by mass, resting, sizes, and a missile ended inside its own contact; on flat ground 2,048 across with the default step of 1/32 second. A footman has collision size 16, so it is 64 tall and a ball of 16 meets it 32 from its middle.</summary>
    public static class ContactCases
    {
        /// <summary>The type id the Struck class is registered with.</summary>
        private const string StruckId = "h0S1";

        /// <summary>The type id the Pushee class is registered with.</summary>
        private const string PusheeId = "h0P1";

        /// <summary>Installs the interactions, physics and terrain packages on fakes, the terrain flat at 0 over 16 by 16 tiles from (0, 0), and registers Struck. Gives the physics fake.</summary>
        private static FakePhysicsWorld Fresh()
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            var corner = GridText.Corner(0.0f, 2);
            var row = "";
            for (var i = 0; i < 17; i++)
            {
                row = row + corner;
            }

            var text = "";
            for (var i = 0; i < 17; i++)
            {
                text = text + row;
            }

            Terrain.Install(new FakeTerrainWorld(), 17, 17, 0.0f, 0.0f, text);
            ObjectTypes.Unit(ObjectTypes.FourCC(StruckId), () => new Struck());
            ObjectTypes.Unit(ObjectTypes.FourCC(PusheeId), () => new Pushee());
            TestLog.Lines.Clear();
            return fake;
        }

        /// <summary>Makes a Struck for a player slot at a place, with a label, and takes it in.</summary>
        private static Struck StruckAt(FakeWorld world, float x, float y, int slot, string label)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC(StruckId), slot);
            world.MoveUnit(handle, x, y);
            var unit = (Struck)Unit.Of(handle)!;
            unit.Label = label;
            return unit;
        }

        /// <summary>Makes a footman of the game for a player slot at a place, and takes it in.</summary>
        private static Unit FootmanAt(FakeWorld world, float x, float y, int slot)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), slot);
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Makes a destructable of a type at a place, and takes it in.</summary>
        private static Destructable BarrelAt(FakeWorld world, string type, float x, float y)
        {
            var handle = world.NewDestructable(ObjectTypes.FourCC(type));
            world.PlaceDestructable(handle, x, y);
            world.PlacedDestructables.Add(handle);
            return Destructable.Of(handle)!;
        }

        /// <summary>Makes an item on the ground at a place, and takes it in.</summary>
        private static Item ItemAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, x, y);
            world.PlacedItems.Add(handle);
            return Item.Of(handle)!;
        }

        /// <summary>Places a shot at a place with a velocity and gives it back.</summary>
        private static Shot ShotAt(Shot shot, float x, float y, float z, float vx, float vy, float vz)
        {
            Missile.Create(shot, x, y, z);
            shot.VX = vx;
            shot.VY = vy;
            shot.VZ = vz;
            return shot;
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Moves the game on a step at a time, at most a number of steps, until a missile has ended.</summary>
        private static void UntilEnded(FakeWorld world, Missile missile, int most)
        {
            for (var i = 0; i < most && !missile.HasEnded; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Moves the game on a step at a time, at most a number of steps, until a shot has had a number of impacts.</summary>
        private static void UntilImpacts(FakeWorld world, Shot shot, int impacts, int most)
        {
            for (var i = 0; i < most && shot.Impacts.Count < impacts; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>What an impact read inside OnImpact: index 0 kind, 1 to 3 the point, 4 to 6 the normal, 7 the speed.</summary>
        private static float Read(Shot shot, int impact, int index)
        {
            return shot.Reads[impact * 8 + index];
        }

        /// <summary>How many lines of the test log are a text.</summary>
        private static int Lines(string text)
        {
            var count = 0;
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                if (TestLog.Lines[i] == text)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>How many times an effect was removed.</summary>
        private static int RemovedTimes(FakePhysicsWorld fake, effect look)
        {
            var count = 0;
            for (var i = 0; i < fake.Removed.Count; i++)
            {
                if (fake.Removed[i] == look)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>How many effects of a model were made.</summary>
        private static int Made(FakePhysicsWorld fake, string model)
        {
            var count = 0;
            foreach (var pair in fake.Models)
            {
                if (pair.Value == model)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Whether a thing is the other of no pair that has not parted.</summary>
        private static bool Unpaired(Actor thing)
        {
            var entry = Tracking.EntryOf(thing);
            return entry == null || entry.Partners == null || entry.Partners.Count == 0;
        }

        /// <summary>Whether a line printed on the screen holds a text.</summary>
        private static bool PrintedWith(FakeWorld world, string text)
        {
            for (var i = 0; i < world.Printed.Count; i++)
            {
                if (world.Printed[i].Contains(text))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>
        /// A bolt of 900 a second fired level at an enemy footman, with the default UnitContact, Stop: OnImpact runs once,
        /// with the missile's owner current, met at the footman's surface with the normal out of it and the speed it closed
        /// at; 75 fire damage is dealt from the source with the missile as the damage's tag; it ends, its effect moved to the
        /// point of contact and removed once, its ImpactModel and ImpactSound played. Its source, which it starts inside, is
        /// not hit.
        /// </summary>
        public static void Stop(FakeWorld world)
        {
            var fake = Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var target = StruckAt(world, 400.0f, 500.0f, 1, "target");
            var bolt = new Shot();
            Expect.True(bolt.UnitContact == Contact.Stop, "UnitContact is Stop by default");
            Expect.True(bolt.DestructableContact == Contact.None && bolt.ItemContact == Contact.None && bolt.MissileContact == Contact.None && bolt.WallContact == Contact.None, "the other kinds are None by default");
            bolt.Model = "bolt.mdl";
            bolt.ImpactModel = "boom.mdl";
            bolt.ImpactSound = "boom.wav";
            bolt.Speed = 900.0f;
            bolt.ImpactDamage = 75.0f;
            bolt.DamageKind = DamageKind.Fire;
            source.Fire(bolt, 400.0f, 500.0f);
            var look = bolt.Effect!;
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it ended at the footman");
            Expect.Equal(1, bolt.Impacts.Count, "OnImpact once");
            Expect.Same(target, bolt.Met[0], "it met the target, not its source");
            Expect.Same(target, bolt.Impacts[0].Unit, "the impact's Unit");
            Expect.True(bolt.Impacts[0].Destructable == null && bolt.Impacts[0].Item == null && bolt.Impacts[0].Missile == null, "and no other kind");
            Within((float)(int)ImpactKind.Unit, Read(bolt, 0, 0), 0.0f, "of a unit");
            Within(384.0f, Read(bolt, 0, 1), 0.01f, "at the footman's surface: x");
            Within(500.0f, Read(bolt, 0, 2), 0.01f, "y");
            Within(60.0f, Read(bolt, 0, 3), 0.01f, "z, its launch height");
            Within(-1.0f, Read(bolt, 0, 4), 0.0001f, "the normal out of the footman: x");
            Within(0.0f, Read(bolt, 0, 5), 0.0001f, "y");
            Within(0.0f, Read(bolt, 0, 6), 0.0001f, "z");
            Within(900.0f, Read(bolt, 0, 7), 0.01f, "the speed it closed at");
            Within(75.0f, bolt.Damages[0], 0.0f, "the impact carried ImpactDamage");
            Expect.Same(Game.Instance, bolt.ImpactedWith, "OnImpact ran with the missile's owner, the game, current");
            Expect.Equal(1, world.Dealt.Count, "damage dealt once");
            Expect.Equal("75 attack:False ranged:False 0 8 0", world.Dealt[0], "75 fire, spells");
            Expect.Same(bolt, target.Tags[0], "with the missile as the tag");
            Expect.Same(source, target.Sources[0], "from the source");
            Expect.Equal(1, RemovedTimes(fake, look), "its effect removed once");
            var place = fake.Places[look];
            Within(384.0f, place[0], 0.01f, "its effect moved to the point of contact first: x");
            Within(500.0f, place[1], 0.01f, "y");
            Within(60.0f, place[2], 0.01f, "z");
            Expect.Equal(1, Made(fake, "boom.mdl"), "its ImpactModel played once");
            Expect.True(fake.Played.Count == 1 && fake.Played[0] == "boom.wav", "its ImpactSound played once");
            Expect.True(Unpaired(target) && Unpaired(source), "every pair parted");
        }

        /// <summary>CanHit is asked once for each thing a missile touches until they part: its source (refused by default), an ally its CanHit refuses, though it flies through it for steps, and the enemy, which it hits. With the default CanHit the ally is hit.</summary>
        public static void CanHit(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var ally = FootmanAt(world, 250.0f, 500.0f, 0);
            var enemy = StruckAt(world, 400.0f, 500.0f, 1, "enemy");
            var bolt = new Shot();
            bolt.Speed = 600.0f;
            bolt.Hit = other => other is Unit unit && bolt.Source!.Player.IsEnemy(unit.Player);
            source.Fire(bolt, 600.0f, 500.0f);
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped at the enemy");
            Expect.Equal(3, bolt.Asked.Count, "asked once for each: " + bolt.Asked.Count);
            Expect.Same(source, bolt.Asked[0], "its source first, which it starts inside");
            Expect.Same(ally, bolt.Asked[1], "then the ally, once though it was inside it for steps");
            Expect.Same(enemy, bolt.Asked[2], "then the enemy");
            Expect.Equal(1, bolt.Met.Count, "one impact");
            Expect.Same(enemy, bolt.Met[0], "with the enemy");

            var plain = new Shot();
            plain.Speed = 600.0f;
            source.Fire(plain, 600.0f, 500.0f);
            UntilEnded(world, plain, 64);
            Expect.Equal(2, plain.Asked.Count, "the default: asked about the source and the ally");
            Expect.Same(source, plain.Asked[0], "the source");
            Expect.Same(ally, plain.Met[0], "the default CanHit lets it hit the ally");
            Expect.Equal(1, plain.Met.Count, "and only it");
        }

        /// <summary>
        /// The order at a contact: the missile's OnImpact, its component's, then the unit's OnStruck on its class and its
        /// buff (its component that writes none is not called), then the damage. A buff that sets the outcome to Bounce and
        /// the damage to 0, as the design's Reflect: the missile bounces back and nothing is dealt.
        /// </summary>
        public static void Order(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var target = StruckAt(world, 400.0f, 500.0f, 1, "unit");
            target.Apply(new StruckBuff());
            var bolt = new Shot();
            bolt.Watch();
            bolt.Speed = 600.0f;
            bolt.ImpactDamage = 10.0f;
            source.Fire(bolt, 600.0f, 500.0f);
            TestLog.Lines.Clear();
            UntilEnded(world, bolt, 64);
            var seen = new List<string>();
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                if (TestLog.Lines[i] != "shot can-hit")
                {
                    seen.Add(TestLog.Lines[i]);
                }
            }

            Expect.Equal("shot impact|watch impact|unit struck|buff struck|unit damaged", string.Join("|", seen), "the order");
            Expect.Same(Game.Instance, target.StruckWith, "OnStruck ran with the missile's owner current");
            Expect.Equal(0, world.Printed.Count, "nothing was reported: the component that writes no OnStruck was not called");

            var other = FootmanAt(world, 100.0f, 800.0f, 0);
            var reflector = StruckAt(world, 400.0f, 800.0f, 1, "reflector");
            var reflect = new StruckBuff();
            reflect.Label = "reflect";
            reflect.Striking = impact =>
            {
                impact.Contact = Contact.Bounce;
                impact.Damage = 0.0f;
            };
            reflector.Apply(reflect);
            var second = new Shot();
            second.Speed = 600.0f;
            second.ImpactDamage = 10.0f;
            other.Fire(second, 600.0f, 800.0f);
            UntilImpacts(world, second, 1, 64);
            Expect.Equal(1, second.Impacts.Count, "it met the reflector");
            Expect.True(!second.HasEnded, "and bounced instead of stopping");
            Within(-300.0f, second.VX, 0.01f, "back, at half the speed into it");
            Expect.Equal(1, world.Dealt.Count, "and nothing was dealt");
            Expect.Equal(0, reflector.Tags.Count, "the reflector took nothing");
        }

        /// <summary>A Bounce off a footman's side, hit 16 off its middle: the speed into the footman is reversed and halved (Bounciness 0.5) and the speed along its surface kept; it meets it once, at the closing speed, and flies on.</summary>
        public static void Bounce(FakeWorld world)
        {
            Fresh();
            FootmanAt(world, 400.0f, 500.0f, 1);
            var ball = new Shot();
            ball.UnitContact = Contact.Bounce;
            ShotAt(ball, 100.0f, 516.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            UntilImpacts(world, ball, 1, 64);
            Expect.Equal(1, ball.Impacts.Count, "it met the footman");
            var nx = Read(ball, 0, 4);
            var ny = Read(ball, 0, 5);
            Within(-0.8660254f, nx, 0.0001f, "the normal out of its side: x");
            Within(0.5f, ny, 0.0001f, "y");
            Within(0.0f, Read(ball, 0, 6), 0.0001f, "level");
            Within(519.615f, Read(ball, 0, 7), 0.01f, "it closed at the speed into the side");
            Within(400.0f - 27.71281f, ball.X, 0.01f, "it is put where it touched: x");
            Within(516.0f, ball.Y, 0.01f, "y");
            Within(259.8076f, ball.VX * nx + ball.VY * ny, 0.01f, "the speed into it reversed and halved");
            Within(600.0f * -ny, ball.VX * -ny + ball.VY * nx, 0.01f, "the speed along it kept");
            Steps(world, 20);
            Expect.Equal(1, ball.Impacts.Count, "it flies off and does not meet it again");
            Expect.True(!ball.HasEnded, "a bounce does not end it");
        }

        /// <summary>
        /// Pass reacts once each time it enters a thing, not again until it has left it: through a, which it is inside for
        /// steps, once; back into a from just outside it (still within the checks' range, so the pair has not parted), once
        /// more; then through a, b and c, each once, in the order met; back through c after leaving it, once more.
        /// </summary>
        public static void Pass(FakeWorld world)
        {
            Fresh();
            var a = StruckAt(world, 200.0f, 500.0f, 1, "a");
            var b = StruckAt(world, 500.0f, 500.0f, 1, "b");
            var c = StruckAt(world, 800.0f, 500.0f, 1, "c");
            var ball = new Shot();
            ball.UnitContact = Contact.Pass;
            ShotAt(ball, 50.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            for (var i = 0; i < 32 && ball.X <= 250.0f; i++)
            {
                Steps(world, 1);
            }

            Expect.Equal(1, ball.Met.Count, "through a: once, though inside it for steps");
            Expect.True(ball.X > 232.0f && ball.X < 200.0f + 98.75f, "it has left a's shape but is within the range of its checks: at " + ball.X);
            ball.VX = -600.0f;
            Steps(world, 7);
            Expect.Equal(2, ball.Met.Count, "back into a after leaving its shape: once more");
            Expect.Same(a, ball.Met[1], "a");
            Expect.True(ball.X < 168.0f, "and out the other side: at " + ball.X);
            ball.VX = 600.0f;
            Steps(world, 45);
            Expect.Equal(5, ball.Met.Count, "through a, b and c");
            Expect.Same(a, ball.Met[2], "a first");
            Expect.Same(b, ball.Met[3], "then b");
            Expect.Same(c, ball.Met[4], "then c");
            Expect.Equal(3, Lines("a struck"), "a's OnStruck once each time");
            Expect.True(ball.X > 950.0f, "it flew on through them: at " + ball.X);
            Within(600.0f, ball.VX, 0.01f, "at its speed");
            ball.VX = -600.0f;
            Steps(world, 20);
            Expect.Equal(6, ball.Met.Count, "back through c after leaving it: once more");
            Expect.Same(c, ball.Met[5], "c");
            Expect.Equal(2, Lines("c struck"), "c's OnStruck twice in all");
        }

        /// <summary>
        /// A bouncing bomb with damage dropped from 300 onto a standing footman's top (80 for its ball): it deals its damage at
        /// its real bounces, a few, then settles on the top and deals no more, with no impact, sitting at 80. One dropped with
        /// a little speed along the ground settles, rolls off the top and stops on the ground, dealing no more.
        /// </summary>
        public static void SettlesOnTop(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 100.0f, 0);
            var footman = StruckAt(world, 500.0f, 500.0f, 1, "footman");
            var bomb = new Shot();
            bomb.UnitContact = Contact.Bounce;
            bomb.Gravity = 800.0f;
            bomb.ImpactDamage = 10.0f;
            bomb.Source = source;
            ShotAt(bomb, 500.0f, 500.0f, 300.0f, 0.0f, 0.0f, 0.0f);
            Steps(world, 96);
            var dealt = footman.Tags.Count;
            Expect.True(dealt >= 2 && dealt <= 6, "a few real bounces dealt their damage: " + dealt);
            Expect.Equal(dealt, bomb.Impacts.Count, "one impact each");
            Steps(world, 64);
            Expect.Equal(dealt, footman.Tags.Count, "no more damage while it sits");
            Expect.Equal(dealt, bomb.Impacts.Count, "and no impact");
            Within(80.0f, bomb.Z, 0.5f, "it sits on the top");
            Expect.True(!bomb.HasEnded, "a bounce does not end it");

            var other = StruckAt(world, 500.0f, 800.0f, 1, "other");
            var roller = new Shot();
            roller.UnitContact = Contact.Bounce;
            roller.Gravity = 800.0f;
            roller.ImpactDamage = 10.0f;
            roller.Source = source;
            ShotAt(roller, 500.0f, 800.0f, 300.0f, 20.0f, 0.0f, 0.0f);
            Steps(world, 192);
            Expect.True(roller.HasEnded, "it rolled off the top and stopped on the ground");
            Expect.True(roller.X > 532.0f, "beyond the top's edge: at " + roller.X);
            Expect.True(other.Tags.Count >= 2 && other.Tags.Count <= 6, "it dealt damage at a few real bounces only: " + other.Tags.Count);
            Expect.Equal(other.Tags.Count + 1, roller.Impacts.Count, "an impact for each, and one with the ground");
        }

        /// <summary>A bolt moved across a unit by setting its X in its own OnTick, after its step, is not swept along the jump: it does not hit the unit, and flies on.</summary>
        public static void Teleport(FakeWorld world)
        {
            Fresh();
            StruckAt(world, 500.0f, 500.0f, 1, "unit");
            var bolt = new Shot();
            var jumped = false;
            bolt.Ticking = () =>
            {
                if (!jumped && bolt.X > 390.0f)
                {
                    jumped = true;
                    bolt.X = 590.0f;
                }
            };
            bolt.TickEachStep();
            ShotAt(bolt, 300.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            Steps(world, 20);
            Expect.True(jumped, "it was moved across the unit");
            Expect.Equal(0, bolt.Impacts.Count, "it did not hit the unit it jumped over");
            Expect.True(!bolt.HasEnded && bolt.X > 700.0f, "it flies on: at " + bolt.X);
        }

        /// <summary>A bolt at 3,000 a second (93.75 a step) through a footman's middle, and one 28 off its middle: one contact each; one 33 off it misses.</summary>
        public static void Fast(FakeWorld world)
        {
            Fresh();
            var middle = FootmanAt(world, 500.0f, 300.0f, 1);
            var side = FootmanAt(world, 500.0f, 500.0f, 1);
            FootmanAt(world, 500.0f, 700.0f, 1);
            var through = new Shot();
            through.UnitContact = Contact.Pass;
            ShotAt(through, 50.0f, 300.0f, 32.0f, 3000.0f, 0.0f, 0.0f);
            var grazing = new Shot();
            grazing.UnitContact = Contact.Pass;
            ShotAt(grazing, 50.0f, 528.0f, 32.0f, 3000.0f, 0.0f, 0.0f);
            var wide = new Shot();
            wide.UnitContact = Contact.Pass;
            ShotAt(wide, 50.0f, 733.0f, 32.0f, 3000.0f, 0.0f, 0.0f);
            Steps(world, 10);
            Expect.True(through.X > 900.0f, "they flew past");
            Expect.Equal(1, through.Met.Count, "through the middle: one contact");
            Expect.Same(middle, through.Met[0], "with the footman");
            Within(468.0f, Read(through, 0, 1) - 16.0f, 0.01f, "met where the ball first touched it, between two steps' places");
            Expect.Equal(1, grazing.Met.Count, "28 off the middle: one contact");
            Expect.Same(side, grazing.Met[0], "with the footman");
            Expect.Equal(0, wide.Met.Count, "33 off the middle: none");
        }

        /// <summary>A missile that starts inside a unit meets it at its first step, once; one at a unit's middle with no velocity, whose normal would be zero, gets a normal of length 1 and no NaN.</summary>
        public static void Inside(FakeWorld world)
        {
            Fresh();
            FootmanAt(world, 500.0f, 500.0f, 1);
            var ball = new Shot();
            ball.UnitContact = Contact.Pass;
            ShotAt(ball, 500.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            Steps(world, 1);
            Expect.Equal(1, ball.Impacts.Count, "met at the first step");
            Within(-1.0f, Read(ball, 0, 4), 0.0001f, "a normal against its flight, as the middle gives none: x");
            Within(0.0f, Read(ball, 0, 5), 0.0001f, "y");
            Within(0.0f, Read(ball, 0, 6), 0.0001f, "z");
            Steps(world, 10);
            Expect.Equal(1, ball.Impacts.Count, "once");

            FootmanAt(world, 500.0f, 800.0f, 1);
            var still = new Shot();
            still.UnitContact = Contact.Bounce;
            still.Mass = 30.0f;
            ShotAt(still, 500.0f, 800.0f, 32.0f, 0.0f, 0.0f, 0.0f);
            Steps(world, 4);
            Expect.Equal(1, still.Impacts.Count, "the still one met it once");
            var nx = Read(still, 0, 4);
            var ny = Read(still, 0, 5);
            var nz = Read(still, 0, 6);
            Expect.True(!float.IsNaN(nx) && !float.IsNaN(ny) && !float.IsNaN(nz), "no NaN in the normal");
            Within(1.0f, MathF.Sqrt(nx * nx + ny * ny + nz * nz), 0.0001f, "a normal of length 1");
            Expect.True(!float.IsNaN(still.VX) && !float.IsNaN(still.VY) && !float.IsNaN(still.VZ), "no NaN in its velocity");
            Expect.True(!float.IsNaN(still.X) && !float.IsNaN(still.Y) && !float.IsNaN(still.Z), "nor in its place");
        }

        /// <summary>Grazing a footman's top (64 up), its ball's lowest point 2 under it, at 600 a second: one contact; 1 over it, none. Falling onto the top: one contact, with the normal up, at its height.</summary>
        public static void Top(FakeWorld world)
        {
            Fresh();
            FootmanAt(world, 500.0f, 300.0f, 1);
            FootmanAt(world, 500.0f, 500.0f, 1);
            FootmanAt(world, 500.0f, 700.0f, 1);
            var grazing = new Shot();
            grazing.UnitContact = Contact.Pass;
            ShotAt(grazing, 300.0f, 300.0f, 78.0f, 600.0f, 0.0f, 0.0f);
            var over = new Shot();
            over.UnitContact = Contact.Pass;
            ShotAt(over, 300.0f, 500.0f, 81.0f, 600.0f, 0.0f, 0.0f);
            var falling = new Shot();
            falling.UnitContact = Contact.Pass;
            falling.GroundContact = Contact.None;
            ShotAt(falling, 500.0f, 700.0f, 200.0f, 0.0f, 0.0f, -600.0f);
            Steps(world, 20);
            Expect.Equal(1, grazing.Impacts.Count, "grazing the top: one contact");
            Expect.Equal(0, over.Impacts.Count, "over it: none");
            Expect.Equal(1, falling.Impacts.Count, "falling onto it: one contact");
            Within(0.0f, Read(falling, 0, 4), 0.0001f, "the normal up: x");
            Within(0.0f, Read(falling, 0, 5), 0.0001f, "y");
            Within(1.0f, Read(falling, 0, 6), 0.0001f, "z");
            Within(64.0f, Read(falling, 0, 3), 0.01f, "at the top");
        }

        /// <summary>A ball resting on the ground pauses its contact checks: a unit put onto it is not asked about or met; given a velocity, it checks again and meets the unit.</summary>
        public static void Resting(FakeWorld world)
        {
            Fresh();
            var ball = new Shot();
            ball.GroundContact = Contact.Slide;
            ball.UnitContact = Contact.Pass;
            ShotAt(ball, 300.0f, 500.0f, 16.0f, 0.0f, 0.0f, -1.0f);
            Steps(world, 4);
            Expect.True(ball.resting, "it rests");
            Expect.True(ball.unitChecks != null && ball.unitChecks.IsPaused, "its unit checks are paused");
            var unit = FootmanAt(world, 600.0f, 500.0f, 1);
            Steps(world, 2);
            world.MoveUnit(unit.Handle, 300.0f, 500.0f);
            Steps(world, 8);
            Expect.Equal(0, ball.Asked.Count, "nothing asked while it rests");
            Expect.Equal(0, ball.Impacts.Count, "nothing met");
            ball.VX = 300.0f;
            Steps(world, 3);
            Expect.True(!ball.unitChecks!.IsPaused, "its checks resumed");
            Expect.Equal(1, ball.Impacts.Count, "it met the unit it was under");
            Expect.Same(unit, ball.Met[0], "the unit");
        }

        /// <summary>A barrel type given a radius of 32 and a height of 80 by DestructableSize: a ball 100 up passes over it, one 60 up stops 32 from its middle; a type not sized is 64 around and 250 tall.</summary>
        public static void Sized(FakeWorld world)
        {
            Fresh();
            Physics.DestructableSize(ObjectTypes.FourCC("B001"), 32.0f, 80.0f);
            BarrelAt(world, "B001", 500.0f, 300.0f);
            BarrelAt(world, "B001", 500.0f, 500.0f);
            BarrelAt(world, "B002", 500.0f, 700.0f);
            var high = new Shot();
            high.DestructableContact = Contact.Stop;
            ShotAt(high, 300.0f, 300.0f, 100.0f, 600.0f, 0.0f, 0.0f);
            var low = new Shot();
            low.DestructableContact = Contact.Stop;
            ShotAt(low, 300.0f, 500.0f, 60.0f, 600.0f, 0.0f, 0.0f);
            var plain = new Shot();
            plain.DestructableContact = Contact.Stop;
            ShotAt(plain, 300.0f, 700.0f, 100.0f, 600.0f, 0.0f, 0.0f);
            Steps(world, 24);
            Expect.Equal(0, high.Impacts.Count, "100 up: over the sized barrel");
            Expect.True(!high.HasEnded && high.X > 600.0f, "and on past it");
            Expect.Equal(1, low.Impacts.Count, "60 up: it meets it");
            Expect.True(low.HasEnded, "and stops");
            Within(468.0f, Read(low, 0, 1), 0.01f, "at its radius of 32");
            Expect.Equal(1, plain.Impacts.Count, "a type not sized: met at 100 up");
            Within(436.0f, Read(plain, 0, 1), 0.01f, "at a radius of 64");
            Within((float)(int)ImpactKind.Destructable, Read(plain, 0, 0), 0.0f, "of a destructable");
        }

        /// <summary>
        /// Shoves by mass: a 30-mass ball at 600 into a footman of mass 16, with Bounce: the closing speed shared by mass with
        /// Bounciness 0.5, the footman pushed at 586.96 and the ball slowed to 286.96, the ball's source the pusher. A Stop
        /// shoves as a bounce that keeps nothing: 391.30. A structure is not pushed, and the ball bounces off it as off a wall.
        /// A footman walking at 320 a second into a still ball: its speed, from the pair's last two places, is shared.
        /// </summary>
        public static void Mass(FakeWorld world)
        {
            var fake = Fresh();
            var shooter = FootmanAt(world, 100.0f, 50.0f, 0);
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId), 1);
            world.MoveUnit(handle, 500.0f, 200.0f);
            var footman = (Pushee)Unit.Of(handle)!;
            var ball = new Shot();
            ball.UnitContact = Contact.Bounce;
            ball.Mass = 30.0f;
            ball.Source = shooter;
            ShotAt(ball, 300.0f, 200.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            UntilImpacts(world, ball, 1, 64);
            Expect.Equal(1, ball.Impacts.Count, "it met the footman");
            Expect.Equal(1, footman.Pushers.Count, "OnPushed ran once");
            Expect.Same(shooter, footman.Pushers[0], "with the missile's Source as the pusher");
            Within(586.96f, footman.PushVelocity().X, 0.05f, "the footman pushed by its share");
            Within(0.0f, footman.PushVelocity().Y, 0.001f, "along the normal");
            Within(286.96f, ball.VX, 0.05f, "the ball slowed by its share");

            var stopped = FootmanAt(world, 500.0f, 400.0f, 1);
            var stopper = new Shot();
            stopper.Mass = 30.0f;
            ShotAt(stopper, 300.0f, 400.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            UntilEnded(world, stopper, 64);
            Expect.True(stopper.HasEnded, "a Stop ends it");
            Within(391.30f, stopped.PushVelocity().X, 0.05f, "and shoves as a bounce that keeps nothing");

            var tower = FootmanAt(world, 500.0f, 600.0f, 1);
            fake.Structures.Add(tower.Handle);
            var bouncer = new Shot();
            bouncer.UnitContact = Contact.Bounce;
            bouncer.Mass = 30.0f;
            ShotAt(bouncer, 300.0f, 600.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            UntilImpacts(world, bouncer, 1, 64);
            Expect.True(!tower.IsPushed(), "a structure is not pushed");
            Within(-300.0f, bouncer.VX, 0.01f, "the ball bounces off it as off a wall");

            var walker = FootmanAt(world, 400.0f, 800.0f, 1);
            var still = new Shot();
            still.UnitContact = Contact.Bounce;
            still.Mass = 30.0f;
            ShotAt(still, 500.0f, 800.0f, 32.0f, 0.0f, 0.0f, 0.0f);
            for (var i = 0; i < 20 && still.Impacts.Count == 0; i++)
            {
                world.MoveUnit(walker.Handle, walker.X + 10.0f, 800.0f);
                Steps(world, 1);
            }

            Expect.Equal(1, still.Impacts.Count, "the walker met the ball");
            Within(320.0f, Read(still, 0, 7), 0.01f, "closing at the walker's speed");
            Within(166.96f, still.VX, 0.05f, "the ball knocked on by its share");
            Within(-313.04f, walker.PushVelocity().X, 0.05f, "the walker pushed back by its share");
        }

        /// <summary>A bolt with DestructableContact Stop and damage stops at a barrel, and the damage is dealt to it from the source.</summary>
        public static void DestructableHit(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var barrel = BarrelAt(world, "B003", 500.0f, 500.0f);
            var bolt = new Shot();
            bolt.Speed = 600.0f;
            bolt.UnitContact = Contact.None;
            bolt.DestructableContact = Contact.Stop;
            bolt.ImpactDamage = 40.0f;
            source.Fire(bolt, 700.0f, 500.0f);
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped");
            Expect.Equal(1, bolt.Impacts.Count, "OnImpact once");
            Expect.Same(barrel, bolt.Impacts[0].Destructable, "the impact's Destructable");
            Expect.True(bolt.Impacts[0].Unit == null, "and no Unit");
            Within(436.0f, Read(bolt, 0, 1), 0.01f, "at its surface");
            Expect.Equal(1, world.DealtToDestructables.Count, "damage dealt to it");
            Expect.Equal("40 attack:False ranged:False 0 14 0", world.DealtToDestructables[0], "40, magic and spells");
            Expect.Same(source.Handle, world.DealtToDestructableSources[0], "from the source");
            Expect.Same(barrel.Handle, world.DealtToDestructableTargets[0], "to the barrel");
            Expect.Equal(0, world.Dealt.Count, "nothing to a unit");
        }

        /// <summary>An item hit runs OnImpact only: with damage set and a source, nothing is dealt.</summary>
        public static void ItemHit(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var item = ItemAt(world, 500.0f, 500.0f);
            var bolt = new Shot();
            bolt.Speed = 600.0f;
            bolt.LaunchHeight = 20.0f;
            bolt.UnitContact = Contact.None;
            bolt.ItemContact = Contact.Stop;
            bolt.ImpactDamage = 40.0f;
            source.Fire(bolt, 700.0f, 500.0f);
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped");
            Expect.Equal(1, bolt.Impacts.Count, "OnImpact once");
            Expect.Same(item, bolt.Impacts[0].Item, "the impact's Item");
            Within((float)(int)ImpactKind.Item, Read(bolt, 0, 0), 0.0f, "of an item");
            Within(460.0f, Read(bolt, 0, 1), 0.01f, "at its radius of 40");
            Expect.Equal(0, world.Dealt.Count + world.DealtToDestructables.Count, "nothing dealt");
        }

        /// <summary>A missile with damage and no source deals none, and a development build reports it (a release build, as the Lua cases are built, does not).</summary>
        public static void NoSource(FakeWorld world)
        {
            Fresh();
            var target = StruckAt(world, 500.0f, 500.0f, 1, "target");
            var bolt = new Shot();
            bolt.ImpactDamage = 50.0f;
            ShotAt(bolt, 300.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped at the unit");
            Expect.Equal(0, world.Dealt.Count, "nothing dealt");
            Expect.Equal(0, target.Tags.Count, "the unit took nothing");
#if DEBUG
            Expect.True(PrintedWith(world, "ImpactDamage"), "a development build reports it");
#else
            Expect.True(!PrintedWith(world, "ImpactDamage"), "a release build does not report it");
#endif
        }

        /// <summary>
        /// A missile ended inside its own contact: by End in OnImpact, by a Stop set while a Pass reacts, by its target's
        /// OnStruck, by its source (its owner) removed inside the damage. No reaction runs after the end, the damage is dealt
        /// at most once, no outcome follows, every pair parts, and its effect is removed once. A source killed inside the
        /// damage does not end it: it stops as set.
        /// </summary>
        public static void EndInReaction(FakeWorld world)
        {
            var fake = Fresh();

            // End in OnImpact.
            var source = FootmanAt(world, 100.0f, 100.0f, 0);
            var target = StruckAt(world, 400.0f, 100.0f, 1, "first");
            target.Apply(new StruckBuff());
            var bolt = new Shot();
            bolt.Model = "bolt.mdl";
            bolt.ImpactModel = "boom.mdl";
            bolt.Speed = 600.0f;
            bolt.ImpactDamage = 10.0f;
            bolt.Watch();
            bolt.Impacting = impact => bolt.End();
            source.Fire(bolt, 600.0f, 100.0f);
            var look = bolt.Effect!;
            TestLog.Lines.Clear();
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "End in OnImpact ended it");
            Expect.Equal(1, Lines("shot impact"), "OnImpact once");
            Expect.Equal(0, Lines("watch impact") + Lines("first struck") + Lines("buff struck") + Lines("first damaged"), "no reaction after the end");
            Expect.Equal(0, world.Dealt.Count, "no damage");
            Expect.Equal(0, Made(fake, "boom.mdl"), "no outcome: no ImpactModel");
            Expect.Equal(1, RemovedTimes(fake, look), "its effect removed once");
            Expect.True(Unpaired(target) && Unpaired(source), "every pair parted");
            Expect.True(bolt.unitChecks!.HasEnded, "its unit checks ended with it");

            // A Stop set while a Pass reacts.
            var a = StruckAt(world, 400.0f, 300.0f, 1, "a");
            var b = StruckAt(world, 600.0f, 300.0f, 1, "b");
            a.Striking = impact => impact.Contact = Contact.Stop;
            var passing = new Shot();
            passing.Model = "pass.mdl";
            passing.ImpactModel = "pop.mdl";
            passing.UnitContact = Contact.Pass;
            ShotAt(passing, 200.0f, 300.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            var passingLook = passing.Effect!;
            UntilEnded(world, passing, 64);
            Steps(world, 16);
            Expect.True(passing.HasEnded, "a Stop set while a Pass reacts ends it");
            Expect.Equal(1, passing.Met.Count, "it met a only");
            Expect.Equal(0, Lines("b struck"), "b never reacted");
            Expect.Equal(1, Made(fake, "pop.mdl"), "its ImpactModel once");
            Expect.Equal(1, RemovedTimes(fake, passingLook), "its effect removed once");
            Expect.True(Unpaired(a) && Unpaired(b), "every pair parted");

            // The target's OnStruck ends it.
            var ender = StruckAt(world, 400.0f, 500.0f, 1, "ender");
            ender.Apply(new StruckBuff { Label = "after" });
            var struck = new Shot();
            struck.Model = "struck.mdl";
            struck.ImpactModel = "crack.mdl";
            struck.ImpactDamage = 10.0f;
            ender.Striking = impact => struck.End();
            ShotAt(struck, 200.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            var struckLook = struck.Effect!;
            var dealt = world.Dealt.Count;
            UntilEnded(world, struck, 64);
            Expect.True(struck.HasEnded, "its target's OnStruck ended it");
            Expect.Equal(0, Lines("after struck"), "no OnStruck after the end");
            Expect.Equal(dealt, world.Dealt.Count, "no damage");
            Expect.Equal(0, Made(fake, "crack.mdl"), "no outcome");
            Expect.Equal(1, RemovedTimes(fake, struckLook), "its effect removed once");
            Expect.True(Unpaired(ender), "every pair parted");

            // The source, its owner, removed inside the damage.
            var owner = FootmanAt(world, 100.0f, 700.0f, 0);
            var victim = StruckAt(world, 400.0f, 700.0f, 1, "victim");
            victim.Damaging = damage => owner.Remove();
            var owned = new Shot();
            owned.Model = "owned.mdl";
            owned.UnitContact = Contact.Bounce;
            owned.Speed = 600.0f;
            owned.ImpactDamage = 10.0f;
            owner.Fire(owned, 600.0f, 700.0f, owner);
            var ownedLook = owned.Effect!;
            dealt = world.Dealt.Count;
            UntilEnded(world, owned, 64);
            Expect.True(owned.HasEnded, "its source, its owner, removed inside the damage ended it");
            Expect.Equal(dealt + 1, world.Dealt.Count, "the damage dealt once");
            Expect.True(owned.VX > 0.0f, "no bounce after the end");
            Expect.Equal(1, RemovedTimes(fake, ownedLook), "its effect removed once");
            Expect.True(Unpaired(victim), "every pair parted");

            // The source killed inside the damage: it does not end the missile.
            var killed = FootmanAt(world, 100.0f, 900.0f, 0);
            var thorns = StruckAt(world, 400.0f, 900.0f, 1, "thorns");
            thorns.Damaging = damage => killed.Kill();
            var shot = new Shot();
            shot.Model = "shot.mdl";
            shot.ImpactModel = "thud.mdl";
            shot.Speed = 600.0f;
            shot.ImpactDamage = 10.0f;
            killed.Fire(shot, 600.0f, 900.0f);
            var shotLook = shot.Effect!;
            dealt = world.Dealt.Count;
            UntilEnded(world, shot, 64);
            Expect.True(!killed.IsAlive, "the source died");
            Expect.True(shot.HasEnded, "it stopped as set");
            Expect.Equal(dealt + 1, world.Dealt.Count, "the damage dealt once");
            Expect.Equal(1, Made(fake, "thud.mdl"), "its ImpactModel once");
            Expect.Equal(1, RemovedTimes(fake, shotLook), "its effect removed once");
        }

        /// <summary>OnImpact and OnStruck run with the missile's owner current, so a timer made in OnImpact of a missile that stops outlives it.</summary>
        public static void Ownership(FakeWorld world)
        {
            Fresh();
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var target = StruckAt(world, 500.0f, 500.0f, 1, "target");
            var fired = 0;
            var bolt = new Shot();
            bolt.Impacting = impact => Timer.After(0.25f, () => fired++);
            Missile.Create(bolt, 300.0f, 500.0f, 32.0f, Player.Two, owner);
            bolt.VX = 600.0f;
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped");
            Expect.Same(owner, bolt.ImpactedWith, "OnImpact ran with the missile's owner current");
            Expect.Same(owner, target.StruckWith, "OnStruck too");
            Steps(world, 10);
            Expect.Equal(1, fired, "the timer made in OnImpact outlived the missile");
        }

        /// <summary>A kind whose setting is None starts no interaction; a missile with UnitContact None flies through a unit, asking nothing. A kind set to None after Create pauses its checks, and set back resumes them.</summary>
        public static void NoneCostsNothing(FakeWorld world)
        {
            Fresh();
            StruckAt(world, 500.0f, 500.0f, 1, "unit");
            var ghost = new Shot();
            ghost.UnitContact = Contact.None;
            ShotAt(ghost, 300.0f, 500.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            Expect.True(ghost.unitChecks == null && ghost.destructableChecks == null && ghost.itemChecks == null, "no interaction started");
            var entry = Tracking.EntryOf(ghost)!;
            Expect.True(entry.Held == null || entry.Held.Count == 0, "it holds none");
            Steps(world, 24);
            Expect.True(ghost.X > 600.0f, "it flew through");
            Expect.Equal(0, ghost.Asked.Count + ghost.Impacts.Count, "asking and meeting nothing");

            var bolt = new Shot();
            ShotAt(bolt, 300.0f, 700.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            Expect.True(bolt.unitChecks != null, "the default starts the unit checks");
            Expect.True(bolt.destructableChecks == null && bolt.itemChecks == null, "and no other");
            Expect.Equal(1, Tracking.EntryOf(bolt)!.Held!.Count, "it holds one");

            var target = StruckAt(world, 700.0f, 900.0f, 1, "target");
            var toggled = new Shot();
            ShotAt(toggled, 300.0f, 900.0f, 32.0f, 600.0f, 0.0f, 0.0f);
            toggled.UnitContact = Contact.None;
            Expect.True(toggled.unitChecks!.IsPaused, "set to None after Create, its checks pause: they cost nothing");
            Steps(world, 4);
            toggled.UnitContact = Contact.Stop;
            Expect.True(!toggled.unitChecks.IsPaused, "set back, they resume");
            UntilEnded(world, toggled, 64);
            Expect.True(toggled.HasEnded, "it stopped");
            Expect.Same(target, toggled.Met[0], "at the unit");
        }

        /// <summary>A source that died while its missile flew still deals the damage, as that unit (the design's assumption; Task 1's probe checks it in the game). One removed from the game deals none, and nothing is reported.</summary>
        public static void DeadSource(FakeWorld world)
        {
            Fresh();
            var source = FootmanAt(world, 100.0f, 500.0f, 0);
            var target = StruckAt(world, 500.0f, 500.0f, 1, "target");
            var bolt = new Shot();
            bolt.Speed = 600.0f;
            bolt.ImpactDamage = 20.0f;
            source.Fire(bolt, 700.0f, 500.0f);
            Steps(world, 2);
            source.Kill();
            UntilEnded(world, bolt, 64);
            Expect.True(bolt.HasEnded, "it stopped");
            Expect.Equal(1, world.Dealt.Count, "the damage dealt");
            Expect.Same(source, target.Sources[0], "from the dead source");

            var gone = FootmanAt(world, 100.0f, 700.0f, 0);
            var other = StruckAt(world, 500.0f, 700.0f, 1, "other");
            var late = new Shot();
            late.Speed = 600.0f;
            late.ImpactDamage = 20.0f;
            gone.Fire(late, 700.0f, 700.0f);
            Steps(world, 2);
            gone.Remove();
            UntilEnded(world, late, 64);
            Expect.True(late.HasEnded, "it stopped");
            Expect.Equal(1, world.Dealt.Count, "a removed source deals nothing");
            Expect.Equal(0, other.Tags.Count, "the unit took nothing");
            Expect.True(!PrintedWith(world, "ImpactDamage"), "and nothing is reported");
        }
    }
}
