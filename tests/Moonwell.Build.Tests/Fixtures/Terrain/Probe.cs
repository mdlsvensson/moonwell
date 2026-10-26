using Moonwell;
using Moonwell.Objects;

namespace TerrainFixture
{
    /// <summary>Makes a footman at the start, and a moment later reads the height and the pathing at its place.</summary>
    public sealed class Probe : GameComponent
    {
        public override void OnGameStart()
        {
            var footman = Unit.Create<Human.Units.Footman>(Player.One, 0f, 0f);
            Timer.After(0.1f, () =>
            {
                if (Terrain.IsWalkable(0f, 0f) && Terrain.Height(0f, 0f) > -10000f)
                {
                    footman.PlaceAt(new Vector2(0f, 0f));
                }
            });
        }
    }
}
