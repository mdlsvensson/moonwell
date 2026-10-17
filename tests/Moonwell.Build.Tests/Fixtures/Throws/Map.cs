using Moonwell;

namespace Throws
{
    public sealed class GameMap : Map
    {
        public GameMap()
        {
            throw new System.InvalidOperationException("no name");
        }
    }
}
