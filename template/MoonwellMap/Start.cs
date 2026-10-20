using Moonwell;
using static Moonwell.Objects.Human.Units;

namespace MoonwellMap
{
    /// <summary>Gives the first player a champion at their start location, and puts three footmen nearby to fight.</summary>
    public sealed class Start : GameComponent
    {
        public override void OnGameStart()
        {
            var player = Player.One;
            var x = player.StartX;
            var y = player.StartY;
            Unit.Create<Champion>(player, x, y, 270f);

            var enemy = Player.NeutralAggressive;
            for (var i = 0; i < 3; i++)
            {
                Unit.Create<Footman>(enemy, x - 200f + i * 200f, y + 600f, 270f);
            }
        }
    }
}
