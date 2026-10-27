using System;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>
    /// Cases for placed bridges: a deck's heights from its shape, turned and scaled; a deck over water, on the ground
    /// beneath; a missile under a deck and onto it, with the deck's normal; a unit standing on one; a push along its rail;
    /// and a bridge that dies. The shape is a small bridge of 3 cells across (with no deck in the cells either side) and 7
    /// along, rising 10 a cell from 100 at its ends to 130 at its middle; the bridge is placed at (500, 500) in
    /// war3map.doo, and the game reports it at (512, 512). The last number of a bridge's line is the ground beneath any
    /// water at its place, as the build writes it.
    /// </summary>
    public static class DeckCases
    {
        /// <summary>The type id of the bridge whose type keeps its fixed rotation.</summary>
        private const string FixedId = "Ytb1";

        /// <summary>The type id of a bridge whose type turns with its facing (fixed rotation -1).</summary>
        private const string TurningId = "Ytb2";

        /// <summary>The type id the Pushee class is registered with.</summary>
        private const string PusheeId = "h0P1";

        /// <summary>The shape, as the build writes it: cell, columns, rows, the origin's column and row, then the values row by row from the south.</summary>
        private const string Shape = "32 5 7 2 3\n"
            + ". 100 100 100 .\n"
            + ". 110 110 110 .\n"
            + ". 120 120 120 .\n"
            + ". 130 130 130 .\n"
            + ". 120 120 120 .\n"
            + ". 110 110 110 .\n"
            + ". 100 100 100 .\n";

        /// <summary>A shape with no symmetry: 4 cells across and 6 along, each 100 + 10 a row + 1 a column, its place at column 1, row 2.</summary>
        private const string Ramp = "32 4 6 1 2\n"
            + "100 101 102 103\n"
            + "110 111 112 113\n"
            + "120 121 122 123\n"
            + "130 131 132 133\n"
            + "140 141 142 143\n"
            + "150 151 152 153\n";

        /// <summary>Installs the interactions, physics and terrain packages on fakes, the terrain flat over 16 by 16 tiles from (0, 0) at a height, under water when wet, and registers Pushee. Gives the physics fake.</summary>
        private static FakePhysicsWorld Fresh(float height, bool wet, out FakeTerrainWorld terrain)
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            terrain = new FakeTerrainWorld();
            var corner = GridText.Corner(height, 2, false, wet);
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

            Terrain.Install(terrain, 17, 17, 0.0f, 0.0f, text);
            ObjectTypes.Unit(ObjectTypes.FourCC(PusheeId), () => new Pushee());
            return fake;
        }

        /// <summary>Makes a destructable of a type where the game reports it, and gives it as the one found near a place of that type.</summary>
        private static destructable Bridge(FakeWorld world, FakePhysicsWorld fake, string type, float x, float y)
        {
            var handle = world.NewDestructable(ObjectTypes.FourCC(type));
            world.PlaceDestructable(handle, x, y);
            world.PlacedDestructables.Add(handle);
            fake.Placed[ObjectTypes.FourCC(type)] = handle;
            return handle;
        }

        /// <summary>Places the fixed bridge, found at (512, 512), unturned at scale 1, on dry ground at 0, and makes its deck.</summary>
        private static destructable Placed(FakeWorld world, FakePhysicsWorld fake)
        {
            return Placed(world, fake, "0");
        }

        /// <summary>Places the fixed bridge, found at (512, 512), unturned at scale 1, with the ground beneath the build wrote for it, and makes its deck.</summary>
        private static destructable Placed(FakeWorld world, FakePhysicsWorld fake, string ground)
        {
            var handle = Bridge(world, fake, FixedId, 512.0f, 512.0f);
            Physics.Install(FixedId + " 0 500 500 270 1 0 " + ground, Shape);
            return handle;
        }

        /// <summary>Makes a footman of the game at a place, and takes it in.</summary>
        private static Unit FootmanAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), 1);
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Makes a Pushee at a place, and takes it in.</summary>
        private static Pushee PusheeAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC(PusheeId));
            world.MoveUnit(handle, x, y);
            return (Pushee)Unit.Of(handle)!;
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Fails unless two floats are within a tolerance.</summary>
        private static void Within(float expected, float actual, float tolerance, string what)
        {
            Expect.True(Math.Abs(expected - actual) <= tolerance, what + ": expected " + expected + " within " + tolerance + ", was " + actual);
        }

        /// <summary>
        /// A bridge's deck from its shape: GroundHeight reads the deck, on the ground at the place the game reports, not the
        /// one in war3map.doo; between two cells it blends them, and at the deck's edge it reads the edge cell; beside it, past
        /// its ends, and in a cell with no deck it reads the terrain. With a height, a deck above it is not counted.
        /// </summary>
        public static void Height(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(40.0f, false, out terrain);
            Placed(world, fake, "40");
            Expect.Equal(1, Decks.count, "one deck");
            Within(170.0f, Physics.GroundHeight(512.0f, 512.0f), 0.01f, "at its middle: 130 above the ground");
            Within(160.0f, Physics.GroundHeight(512.0f, 544.0f), 0.01f, "a cell along");
            Within(165.0f, Physics.GroundHeight(512.0f, 528.0f), 0.01f, "half a cell along, the two blended");
            Within(140.0f, Physics.GroundHeight(512.0f, 608.0f), 0.01f, "at its north end");
            Within(140.0f, Physics.GroundHeight(512.0f, 416.0f), 0.01f, "at its south end");
            Within(170.0f, Physics.GroundHeight(552.0f, 512.0f), 0.01f, "near its edge: the edge cell");
            Within(170.0f, Physics.GroundHeight(472.0f, 512.0f), 0.01f, "near its other edge");
            Within(40.0f, Physics.GroundHeight(562.0f, 512.0f), 0.01f, "in a cell with no deck: the terrain");
            Within(40.0f, Physics.GroundHeight(700.0f, 512.0f), 0.01f, "beside it");
            Within(40.0f, Physics.GroundHeight(512.0f, 630.0f), 0.01f, "past its end");
            Within(40.0f, Physics.GroundHeight(512.0f, 512.0f, 100.0f), 0.01f, "under it, with a height below it: the terrain");
            Within(170.0f, Physics.GroundHeight(512.0f, 512.0f, 200.0f), 0.01f, "with a height above it: the deck");
            Within(170.0f, Physics.GroundHeight(512.0f, 512.0f, 170.0f), 0.01f, "with its own height: the deck");
        }

        /// <summary>
        /// Two bridges of a shape with no symmetry (each cell 100 + 10 a row + 1 a column, its place at column 1, row 2), so
        /// a wrong turn, a turn the wrong way or a wrong origin each reads otherwise. One whose type turns with its facing, at
        /// facing 0 and scale 1.5, is turned a quarter counterclockwise from facing 270: its rows run west, its columns north,
        /// its cells 48 apart and its heights 1.5 times as high, and its normal leans with it. A fixed-rotation one at facing 0
        /// is not turned.
        /// </summary>
        public static void TurnedScaled(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(0.0f, false, out terrain);
            Bridge(world, fake, TurningId, 512.0f, 512.0f);
            Bridge(world, fake, FixedId, 1536.0f, 1536.0f);
            Physics.Install(TurningId + " 0 500 500 0 1.5 1 0\n" + FixedId + " 0 1530 1530 0 1 0 0", Ramp);
            Expect.Equal(2, Decks.count, "two decks");
            Within(181.5f, Physics.GroundHeight(512.0f, 512.0f), 0.01f, "the turned one's place: 1.5 times its cell, 121");
            Within(183.0f, Physics.GroundHeight(512.0f, 560.0f), 0.01f, "48 north: the next column");
            Within(196.5f, Physics.GroundHeight(464.0f, 512.0f), 0.01f, "48 west: the next row");
            Within(166.5f, Physics.GroundHeight(560.0f, 512.0f), 0.01f, "48 east: the row before");
            Within(229.5f, Physics.GroundHeight(368.0f, 608.0f), 0.01f, "its far corner: the last row and column, 153");
            Within(0.0f, Physics.GroundHeight(656.0f, 512.0f), 0.01f, "east of its first row");
            Within(0.0f, Physics.GroundHeight(512.0f, 416.0f), 0.01f, "south of its first column");
            float nx;
            float ny;
            float nz;
            Ground.Normal(488.0f, 520.0f, 1000.0f, out nx, out ny, out nz);
            var west = 10.0f * 1.5f / 48.0f;
            var north = 1.0f * 1.5f / 48.0f;
            var length = MathF.Sqrt(west * west + north * north + 1.0f);
            Within(west / length, nx, 0.001f, "the normal leans east, down the rows: x");
            Within(-north / length, ny, 0.001f, "and south, down the columns: y");
            Within(1.0f / length, nz, 0.001f, "z");
            Within(121.0f, Physics.GroundHeight(1536.0f, 1536.0f), 0.01f, "the fixed one's place");
            Within(122.0f, Physics.GroundHeight(1568.0f, 1536.0f), 0.01f, "32 east: the next column, whatever its facing");
            Within(131.0f, Physics.GroundHeight(1536.0f, 1568.0f), 0.01f, "32 north: the next row");
            Within(100.0f, Physics.GroundHeight(1504.0f, 1472.0f), 0.01f, "its first cell, south-west");
            Within(143.0f, Physics.GroundHeight(1600.0f, 1600.0f), 0.01f, "its last column, two rows north");
        }

        /// <summary>
        /// A bridge over water stands on the ground beneath it, as the game sets it, not on the water's surface, which is
        /// all the terrain's grid keeps there: the grid is water at -20 for the corners up to y 768 and dry ground at 40 from
        /// y 896, and the build wrote the ground beneath the first bridge as -60. Its deck is that plus its shape, and beside
        /// it the ground is the water. A bridge on dry ground stands on Terrain.Height, as before, not on what the build
        /// wrote, which is rounded to a thousandth.
        /// </summary>
        public static void OverWater(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            var terrain = new FakeTerrainWorld();
            var text = "";
            for (var j = 0; j < 17; j++)
            {
                var corner = j <= 6 ? GridText.Corner(-20.0f, 2, false, true) : GridText.Corner(40.0f, 2, false, false);
                for (var i = 0; i < 17; i++)
                {
                    text = text + corner;
                }
            }

            Terrain.Install(terrain, 17, 17, 0.0f, 0.0f, text);
            Bridge(world, fake, FixedId, 512.0f, 512.0f);
            Bridge(world, fake, TurningId, 1536.0f, 1536.0f);
            Physics.Install(FixedId + " 0 500 500 270 1 0 -60\n" + TurningId + " 0 1530 1530 270 1 1 40.001", Shape);
            Expect.Equal(2, Decks.count, "two decks");
            Within(-20.0f, Terrain.Height(512.0f, 512.0f), 0.01f, "the grid under the first bridge: the water's surface");
            Within(70.0f, Physics.GroundHeight(512.0f, 512.0f), 0.01f, "at its middle: 130 above the ground beneath");
            Within(60.0f, Physics.GroundHeight(512.0f, 544.0f), 0.01f, "a cell along");
            Within(40.0f, Physics.GroundHeight(512.0f, 416.0f), 0.01f, "at its south end: 100 above the ground beneath");
            Within(-20.0f, Physics.GroundHeight(700.0f, 512.0f), 0.01f, "beside it: the water");
            Within(-20.0f, Physics.GroundHeight(512.0f, 512.0f, 60.0f), 0.01f, "under it, with a height below it: the water");
            Expect.True(Physics.GroundHeight(1536.0f, 1536.0f) == 170.0f, "the second bridge, on dry ground: 130 above Terrain.Height, 40; was " + Physics.GroundHeight(1536.0f, 1536.0f));
        }

        /// <summary>A missile with DestructableContact set to Stop flies over a deck, through where the bridge would stand as a destructable 64 around and 250 tall on its deck, and stops at a barrel past it: the bridge is not met as a destructable.</summary>
        public static void NotACylinder(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(0.0f, false, out terrain);
            Placed(world, fake);
            var barrel = world.NewDestructable(ObjectTypes.FourCC("LTbr"));
            world.PlaceDestructable(barrel, 900.0f, 512.0f);
            world.PlacedDestructables.Add(barrel);
            var shot = new Shot();
            shot.DestructableContact = Contact.Stop;
            Missile.Create(shot, 300.0f, 512.0f, 200.0f);
            shot.VX = 600.0f;
            for (var i = 0; i < 48 && !shot.HasEnded; i++)
            {
                world.Advance(Interactions.Step);
            }

            Expect.True(shot.HasEnded, "it ended");
            Expect.Equal(1, shot.Impacts.Count, "one impact");
            Expect.Same(Destructable.Of(barrel), shot.Met[0], "the barrel, not the bridge");
            Within(836.0f, shot.Reads[1], 0.5f, "at the barrel's side, 64 from its middle, past the bridge");
        }

        /// <summary>A missile flying level under a deck passes under it; one dropped onto it from above lands on it, as one coming down onto it from beside it over the low ground does.</summary>
        public static void Under(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(0.0f, false, out terrain);
            Placed(world, fake);
            var low = new Ball();
            low.VX = 600.0f;
            Missile.Create(low, 300.0f, 512.0f, 50.0f);
            Steps(world, 20);
            Expect.Equal(0, low.Impacts, "under the deck: no impact");
            Expect.True(!low.HasEnded && low.X > 600.0f, "it flew on past it; at " + low.X);
            var dropped = new Ball();
            dropped.Gravity = 800.0f;
            dropped.VZ = -100.0f;
            Missile.Create(dropped, 512.0f, 496.0f, 400.0f);
            Steps(world, 40);
            Expect.Equal(1, dropped.Impacts, "dropped onto the deck: one impact");
            Within(125.0f, dropped.Read[3], 0.5f, "on the deck, half a cell from its middle");
            var rise = 10.0f / 32.0f;
            var length = MathF.Sqrt(rise * rise + 1.0f);
            Within(0.0f, dropped.Read[4], 0.001f, "the deck's normal: x");
            Within(-rise / length, dropped.Read[5], 0.001f, "leaning south, down the deck: y");
            Within(1.0f / length, dropped.Read[6], 0.001f, "z");
            var arcing = new Ball();
            arcing.Gravity = 800.0f;
            arcing.VY = 250.0f;
            arcing.VZ = 200.0f;
            Missile.Create(arcing, 512.0f, 200.0f, 600.0f);
            Steps(world, 64);
            Expect.Equal(1, arcing.Impacts, "coming down onto it from over the low ground: one impact");
            Expect.True(arcing.Read[2] > 400.0f && arcing.Read[2] < 624.0f, "on the deck; at y " + arcing.Read[2]);
            Within(Physics.GroundHeight(512.0f, arcing.Read[2]), arcing.Read[3], 0.5f, "at the deck's height there");
            Expect.True(arcing.Read[3] > 90.0f, "not on the ground beneath; at " + arcing.Read[3]);
        }

        /// <summary>A footman on a deck stands on it: a missile falling onto it meets its top 64 above the deck, and Fire from it launches 60 above the deck.</summary>
        public static void UnitOn(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(0.0f, false, out terrain);
            Placed(world, fake);
            var footman = FootmanAt(world, 512.0f, 512.0f);
            var falling = new Shot();
            falling.UnitContact = Contact.Pass;
            falling.GroundContact = Contact.None;
            Missile.Create(falling, 512.0f, 512.0f, 400.0f);
            falling.VZ = -600.0f;
            Steps(world, 20);
            Expect.Equal(1, falling.Impacts.Count, "it met the footman once");
            Within(194.0f, falling.Reads[3], 0.01f, "at its top, 64 above the deck");
            var bolt = new Shot();
            bolt.Speed = 600.0f;
            footman.Fire(bolt, 800.0f, 512.0f);
            Within(190.0f, bolt.Z, 0.01f, "Fire launches 60 above the deck");
            var beside = FootmanAt(world, 800.0f, 800.0f);
            var other = new Shot();
            other.Speed = 600.0f;
            beside.Fire(other, 1000.0f, 800.0f);
            Within(60.0f, other.Z, 0.01f, "and 60 above the ground beside it");
        }

        /// <summary>
        /// A unit pushed across a deck stops at its edge, the rail, with OnSlammed, though the terrain under the deck is not
        /// walkable; pushed along it at a slant, it slides along the rail; a unit beside the deck pushed into its side stops
        /// there too.
        /// </summary>
        public static void PushRail(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(0.0f, false, out terrain);
            Placed(world, fake);
            // The terrain under the deck is not walkable, as the water beneath a bridge reads.
            for (var x = 496.0f; x < 544.0f; x += 32.0f)
            {
                for (var y = 400.0f; y < 624.0f; y += 32.0f)
                {
                    terrain.Block(x, y, FakeTerrainWorld.NoWalk);
                }
            }

            var across = PusheeAt(world, 480.0f, 512.0f);
            across.Knockback(0.0f, 100.0f, 0.5f);
            Steps(world, 20);
            Expect.True(!across.IsPushed(), "its push has ended");
            Expect.True(across.X > 520.0f && across.X < 560.0f, "it crossed the deck and stopped at its east edge; at " + across.X);
            Within(512.0f, across.Y, 0.01f, "along its push");
            Expect.True(across.Slams.Count >= 1, "OnSlammed ran");
            var along = PusheeAt(world, 500.0f, 450.0f);
            along.Knockback(60.0f, 200.0f, 0.5f);
            Steps(world, 20);
            Expect.True(along.X > 520.0f && along.X < 560.0f, "it went to the east edge and stayed on the deck; at x " + along.X);
            Expect.True(along.Y > 520.0f, "it slid north along the rail; at y " + along.Y);
            var outside = PusheeAt(world, 650.0f, 512.0f);
            outside.Knockback(180.0f, 100.0f, 0.5f);
            Steps(world, 20);
            Expect.True(outside.X > 560.0f && outside.X < 620.0f, "beside it, it stopped at the deck's side; at " + outside.X);
        }

        /// <summary>
        /// A ball falling at a slant onto a deck over sloped terrain bounces off the deck's slope, not the terrain's beneath
        /// it: the terrain rises north 20 a tile (its normal leans 0.154 south), the deck's south half rises 10 a cell (its
        /// normal leans 0.298 south) and its north half falls so (0.298 north). The first ball meets the deck where it was
        /// given the terrain's normal before (its contact on the deck to the last bit, and the deck left out); the others,
        /// from further south and slower, meet it along both halves.
        /// </summary>
        public static void SlopeNormal(FakeWorld world)
        {
            Interactions.Install(new FakeInteractionWorld());
            var fake = new FakePhysicsWorld();
            Physics.Install(fake);
            var terrain = new FakeTerrainWorld();
            var text = "";
            for (var j = 0; j < 17; j++)
            {
                var corner = GridText.Corner(20.0f * (j - 4), 2, false, false);
                for (var i = 0; i < 17; i++)
                {
                    text = text + corner;
                }
            }

            Terrain.Install(terrain, 17, 17, 0.0f, 0.0f, text);
            Placed(world, fake);
            var rise = 10.0f / 32.0f;
            var length = MathF.Sqrt(rise * rise + 1.0f);
            var slope = 20.0f / 128.0f;
            Within(-slope / MathF.Sqrt(slope * slope + 1.0f), Terrain.Normal(512.0f, 480.0f).Y, 0.001f, "the terrain's normal beneath leans south less than the deck's");
            for (var k = 0; k < 9; k++)
            {
                var ball = new Ball();
                ball.GroundContact = Contact.Bounce;
                ball.Gravity = 800.0f;
                ball.VY = k == 0 ? 300.0f : 200.0f;
                ball.VZ = -200.0f;
                var first = new float[4];
                ball.Impacting = impact =>
                {
                    if (first[3] == 0.0f)
                    {
                        first[0] = impact.Y;
                        first[1] = impact.Normal.Y;
                        first[2] = impact.Normal.Z;
                        first[3] = impact.Z;
                    }
                };
                var startY = k == 0 ? 300.0f : 296.0f + 16.0f * k;
                Missile.Create(ball, 512.0f, startY, 400.0f);
                Steps(world, 48);
                var what = "the ball from y " + startY + ", met at y " + first[0];
                Expect.True(ball.Impacts >= 1, what + ": it met the deck");
                Within(Physics.GroundHeight(512.0f, first[0]), first[3], 0.01f, what + ": on the deck");
                var south = first[0] < 512.0f;
                Expect.True(first[0] > 420.0f && first[0] < 604.0f && Math.Abs(first[0] - 512.0f) > 1.0f, what + ": between its end rows, off its middle");
                Within((south ? -rise : rise) / length, first[1], 0.001f, what + ": the deck's normal, y");
                Within(1.0f / length, first[2], 0.001f, what + ": z");
                if (k == 0)
                {
                    Within(479.976f, first[0], 0.01f, "the first ball meets the deck where it took the terrain's normal before");
                }

                if (!ball.HasEnded)
                {
                    ball.End();
                }
            }
        }

        /// <summary>
        /// A bridge that dies is watched, and its deck is ground no more at once: GroundHeight reads the water beneath, and a
        /// ball resting on the deck falls onto the water. A bridge that leaves the game takes its deck too. The water's
        /// surface is at -20 and the ground beneath it at -60, which the decks stand on.
        /// </summary>
        public static void Dies(FakeWorld world)
        {
            FakeTerrainWorld terrain;
            var fake = Fresh(-20.0f, true, out terrain);
            var handle = Bridge(world, fake, FixedId, 512.0f, 512.0f);
            var other = Bridge(world, fake, TurningId, 1536.0f, 1536.0f);
            Physics.Install(FixedId + " 0 500 500 270 1 0 -60\n" + TurningId + " 0 1530 1530 270 1 1 -60", Shape);
            Expect.True(world.Watched.Contains(handle) && world.Watched.Contains(other), "their deaths are watched");
            Within(70.0f, Physics.GroundHeight(512.0f, 512.0f), 0.01f, "the deck, 130 above the ground beneath the water");
            var ball = new Ball();
            ball.GroundContact = Contact.Slide;
            ball.Gravity = 800.0f;
            ball.Friction = 1000.0f;
            ball.VZ = -10.0f;
            Missile.Create(ball, 512.0f, 512.0f, 87.0f);
            Steps(world, 32);
            Expect.True(ball.Rested >= 1, "the ball rests on the deck");
            Within(86.0f, ball.Z, 0.5f, "on it");
            Inbound.DestructableDied(handle);
            Expect.Equal(1, Decks.count, "the other deck counts");
            Within(-20.0f, Physics.GroundHeight(512.0f, 512.0f), 0.01f, "GroundHeight reads the water");
            Steps(world, 96);
            Within(-4.0f, ball.Z, 0.5f, "the ball fell onto the water");
            Within(70.0f, Physics.GroundHeight(1536.0f, 1536.0f), 0.01f, "the other deck");
            Destructable.Of(other)!.Remove();
            Expect.Equal(0, Decks.count, "a bridge that leaves the game takes its deck");
            Within(-20.0f, Physics.GroundHeight(1536.0f, 1536.0f), 0.01f, "GroundHeight reads the water there");
        }
    }
}
