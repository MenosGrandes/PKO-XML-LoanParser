Provide a file named *operations.xml*, in which there are occurences of Type node, which should be equal to _Spłata kredytu_.
Inside this node must be a string that will match a regex ```KAPITAŁ: ([0-9,]+)\s+ODSETKI: ([0-9,]+)\s+ODSETKI SKAPIT\.: ([0-9,]+)(?:\s+ODSETKI KARNE: ([0-9,]+))?\s+(\d+)```.

It uses https://github.com/go-echarts/go-echarts to draw charts in HTML, and in addition there are few JS functions injected in runtime ( for zoom handling, and sum for zoomed timespace)
