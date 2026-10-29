using Moonwell;
using Moonwell.Objects;

namespace MathsFixture
{
    /// <summary>Makes a footman at the start and places it 300 along facing 45, unless the wall is in the way.</summary>
    public sealed class Push : GameComponent
    {
        public override void OnGameStart()
        {
            var footman = Unit.Create<Human.Units.Footman>(Player.One, 0f, 0f, Game.Instance);
            var wall = new Polygon(new Vector2(500f, -500f), new Vector2(600f, -500f), new Vector2(600f, 500f), new Vector2(500f, 500f));
            var start = footman.Position();
            var end = start.Offset(45f, 300f);
            if (!Raycast.Polygon(start, end, wall, out _))
            {
                footman.PlaceAt(Vector2.Lerp(start, end, Ease.OutQuad(1f)));
            }
        }
    }
}
