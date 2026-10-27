namespace Moonwell.Testing
{
    /// <summary>Cases for Terrain.Raycast and TerrainHit, on small grids made with GridText: one tile from (0, 0) to (128, 128) unless a case says otherwise.</summary>
    public static class GroundRayCases
    {
        /// <summary>Installs a one-tile grid at (0, 0) on a fake world.</summary>
        private static void OneTile(string text)
        {
            Terrain.Install(new FakeTerrainWorld(), 2, 2, 0f, 0f, text);
        }

        /// <summary>Flat ground at 0 with the top right corner at 256.</summary>
        private static void Hill()
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(256f, 2)));
        }

        /// <summary>A ray at height 100 toward the raised corner hits where the ground reaches 100.</summary>
        public static void HillHit(FakeWorld world)
        {
            Hill();
            var did = Terrain.Raycast(new Vector3(0f, 0f, 100f), new Vector3(128f, 128f, 100f), out var hit);
            Expect.True(did, "the ray hit");
            // Along the diagonal the ground is 256 u squared, which is 100 at u = 0.625, at (80, 80).
            Expect.True(hit.Point.X > 79f && hit.Point.X < 81f, "x within 1 of 80, was " + hit.Point.X);
            Expect.True(hit.Point.Y > 79f && hit.Point.Y < 81f, "y within 1 of 80, was " + hit.Point.Y);
            Expect.Near(100f, hit.Point.Z, "z stays on the ray");
            Expect.True(hit.Fraction > 0.615f && hit.Fraction < 0.635f, "fraction near 0.625, was " + hit.Fraction);
            var off = Terrain.Height(hit.Point.X, hit.Point.Y) - hit.Point.Z;
            Expect.True(off > -1f && off < 1f, "the point is within 1 of the ground, was " + off);
            var normal = Terrain.Normal(hit.Point.X, hit.Point.Y);
            Expect.True(hit.Normal == normal, "the normal is the ground's there");
            Expect.True(hit.Normal.Z > 0f, "the normal faces up");
        }

        /// <summary>A ray at 500 over the same ground misses, and gives an empty hit.</summary>
        public static void Miss(FakeWorld world)
        {
            Hill();
            var did = Terrain.Raycast(new Vector3(0f, 0f, 500f), new Vector3(128f, 128f, 500f), out var hit);
            Expect.True(!did, "the ray missed");
            Expect.True(hit == new TerrainHit(Vector3.Zero, Vector3.Zero, 0f), "an empty hit");
        }

        /// <summary>On a cliff tile (levels 2 and 3, z 0 and 128) a ray at 64 from the low side hits at the midline.</summary>
        public static void Cliff(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(128f, 3), GridText.Corner(0f, 2), GridText.Corner(128f, 3)));
            var did = Terrain.Raycast(new Vector3(-100f, 64f, 64f), new Vector3(228f, 64f, 64f), out var hit);
            Expect.True(did, "the ray hit");
            Expect.True(hit.Point.X > 63f && hit.Point.X < 65f, "near the midline, was " + hit.Point.X);
            Expect.Near(64f, hit.Point.Y, "y");
            Expect.Near(64f, hit.Point.Z, "z");
            Expect.Near(-0.7071f, hit.Normal.X, "normal x");
            Expect.Near(0f, hit.Normal.Y, "normal y");
            Expect.Near(0.7071f, hit.Normal.Z, "normal z");
            Expect.Near(164f / 328f, hit.Fraction, "fraction");
        }

        /// <summary>A segment that starts under the ground hits at its start, with the normal straight up; one on the ground hits only when it goes down.</summary>
        public static void StartsUnder(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
            var did = Terrain.Raycast(new Vector3(30f, 40f, -10f), new Vector3(100f, 100f, 200f), out var hit);
            Expect.True(did, "the ray hit");
            Expect.Near(0f, hit.Fraction, "fraction");
            Expect.True(hit.Point == new Vector3(30f, 40f, -10f), "at the start");
            Expect.True(hit.Normal == Vector3.UnitZ, "straight up");
            Expect.True(!Terrain.Raycast(new Vector3(30f, 40f, 0f), new Vector3(30f, 40f, 50f), out _), "on the ground and rising: no hit");
            Expect.True(Terrain.Raycast(new Vector3(30f, 40f, 0f), new Vector3(30f, 40f, -50f), out var onGround), "on the ground and going down: a hit");
            Expect.Near(0f, onGround.Fraction, "on the ground and going down: fraction");
        }

        /// <summary>A segment of no length hits when it is under the ground and misses when it is over it.</summary>
        public static void ZeroLength(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
            var under = new Vector3(50f, 50f, -5f);
            var over = new Vector3(50f, 50f, 5f);
            Expect.True(Terrain.Raycast(under, under, out var hit), "under: hit");
            Expect.Near(0f, hit.Fraction, "under: fraction");
            Expect.True(hit.Normal == Vector3.UnitZ, "under: straight up");
            Expect.True(!Terrain.Raycast(over, over, out _), "over: miss");
        }

        /// <summary>A ray from above straight down, and one straight up from below, behave: down lands on the ground.</summary>
        public static void Vertical(FakeWorld world)
        {
            Hill();
            var did = Terrain.Raycast(new Vector3(64f, 64f, 500f), new Vector3(64f, 64f, -100f), out var hit);
            Expect.True(did, "down hits");
            Expect.Near(64f, hit.Point.X, "x");
            Expect.Near(64f, hit.Point.Y, "y");
            Expect.Near(Terrain.Height(64f, 64f), hit.Point.Z, "at the ground's height");
            Expect.Near((500f - Terrain.Height(64f, 64f)) / 600f, hit.Fraction, "fraction");
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 100f), new Vector3(64f, 64f, 500f), out _), "up from above misses");
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 500f), new Vector3(64f, 64f, 400f), out _), "down but short of it misses");
        }

        /// <summary>A ray from inside to far outside over flat ground at 50 misses: the edge reads flat out there.</summary>
        public static void LeavesMap(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 50f), new Vector3(5000f, -3000f, 50f), out _), "the ray left the map and missed");
        }

        /// <summary>A long ray that dips into the ground in a later span is found, and not before it.</summary>
        public static void LongRay(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
            var did = Terrain.Raycast(new Vector3(0f, 0f, 100f), new Vector3(1000f, 0f, -900f), out var hit);
            Expect.True(did, "the ray hit");
            Expect.Near(100f, hit.Point.X, "down 1 for 1 reaches the ground at x 100");
            Expect.Near(0f, hit.Point.Z, "on the ground");
            Expect.Near(0.1f, hit.Fraction, "fraction");
        }

        /// <summary>A segment that ends at infinity, or at a number that is not one, gives up at once and misses; it does not walk on for ever.</summary>
        public static void NotFinite(FakeWorld world)
        {
            OneTile(GridText.Tile(GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2), GridText.Corner(0f, 2)));
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 50f), new Vector3(float.PositiveInfinity, 64f, 50f), out var infinite), "an infinite end missed");
            Expect.True(infinite == new TerrainHit(Vector3.Zero, Vector3.Zero, 0f), "an infinite end: an empty hit");
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, 50f), new Vector3(float.NaN, 64f, 50f), out var notANumber), "a NaN end missed");
            Expect.True(notANumber == new TerrainHit(Vector3.Zero, Vector3.Zero, 0f), "a NaN end: an empty hit");
            Expect.True(!Terrain.Raycast(new Vector3(float.NaN, 64f, 50f), new Vector3(100f, 64f, 50f), out _), "a NaN start missed");
            Expect.True(!Terrain.Raycast(new Vector3(64f, 64f, float.NaN), new Vector3(100f, 64f, 50f), out _), "a NaN height missed");
        }

        /// <summary>TerrainHit has ==, Equals, a hash and text, as RayHit.</summary>
        public static void HitValues(FakeWorld world)
        {
            var hit = new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.5f);
            Expect.True(hit == new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.5f), "hit ==");
            Expect.True(hit != new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.25f), "hit != fraction");
            Expect.True(hit != new TerrainHit(new Vector3(0f, 1f, 5f), new Vector3(0f, 0f, 1f), 0.5f), "hit != point");
            Expect.True(hit != new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(1f, 0f, 0f), 0.5f), "hit != normal");
            Expect.True(hit.Equals(new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.5f)), "hit Equals");
            Expect.True(hit.Equals((object)new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.5f)), "hit Equals(object)");
            Expect.True(!hit.Equals("x"), "hit not Equals(\"x\")");
            Expect.Equal(hit.GetHashCode(), new TerrainHit(new Vector3(0f, 0f, 5f), new Vector3(0f, 0f, 1f), 0.5f).GetHashCode(), "hit hash");
            Expect.Equal("((0, 0, 5), (0, 0, 1), 0.5)", hit.ToString(), "hit text");
        }

        /// <summary>Raycast before Install throws, as every read of the package does.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            Terrain.Uninstall();
            Expect.Throws<System.InvalidOperationException>(() =>
            {
                Terrain.Raycast(new Vector3(0f, 0f, 5f), new Vector3(10f, 0f, 5f), out var hit);
            }, "Raycast before Install");
        }
    }
}
