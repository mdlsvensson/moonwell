namespace Moonwell.Testing
{
    /// <summary>A case from the full audit's finding F9: a unit told to face its own place keeps its facing.</summary>
    public static class FaceOwnPlaceCases
    {
        /// <summary>FaceToward and FaceNow toward the unit's own place leave it facing as it was (they turned it to 0, east).</summary>
        public static void KeepsFacing(FakeWorld world)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            unit.PlaceAt(100f, 50f);
            unit.FaceNow(90f);
            world.UnitCalls.Clear();
            unit.FaceToward(new Vector2(100f, 50f));
            unit.FaceNow(new Vector2(100f, 50f));
            Expect.Equal(0, world.UnitCalls.Count, "calls that turned the unit");
            Expect.Near(90f, unit.Facing, "still facing 90");
            unit.FaceNow(new Vector2(100f, 40f));
            Expect.Near(270f, unit.Facing, "a point beside it still turns it");
        }
    }
}
