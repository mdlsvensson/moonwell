using Moonwell;
using Moonwell.Objects;
using MoonwellMap.Heroes;

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
            Unit.Create<Champion>(player, x, y);

            var enemy = Player.NeutralAggressive;
            for (var i = 0; i < 3; i++)
            {
                Unit.Create<Human.Units.Footman>(enemy, x - 200f + i * 200f, y + 600f);
            }
        }
    }
}
