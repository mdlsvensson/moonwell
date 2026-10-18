using Moonwell;

namespace Objects
{
    public sealed class ObjectsMap : Map
    {
        public ObjectsMap()
        {
            Name = "Objects map";
            Components.Add(new Report());
        }
    }
}
