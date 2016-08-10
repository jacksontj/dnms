'''
Create a visual representation of the various DAGs defined
'''

import sys
import requests
import networkx as nx
import matplotlib.pyplot as plt


if __name__ == '__main__':
    g = nx.DiGraph()
    labels = {
        'edges': {},
        'nodes': {},
    }

    for routeKey, routeMap in requests.get(sys.argv[1]).json().iteritems():
        for i, node in enumerate(routeMap['Path']):
            g.add_node(node['Name'])
            labels['nodes'][node['Name']] = node['Name']
            if i - 1 >= 0:
                g.add_edge(routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])
                labels['edges'][(routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])] = (routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])


    pos = nx.drawing.spring_layout(
        g,
        scale=10.0,
    )
    nx.draw_networkx(
        g,
        pos=pos,
        with_labels=True,
        font_size=8,
    )

    # write out the graph
    plt.savefig(
        'topology.png',
        dpi=400.0,
    )
    plt.show()  # in case people have the required libraries to make it happen
