using Moonwell;

namespace CallsGame
{
    public sealed class GameMap : Map
    {
        public GameMap()
        {
            Natives.GetRandomInt(1, 2);
        }
    }
}
