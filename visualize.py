'''
Create a visual representation of the various DAGs defined
'''

import requests
import networkx as nx
import matplotlib.pyplot as plt


if __name__ == '__main__':
    g = nx.DiGraph()
    labels = {
        'edges': {},
        'nodes': {},
    }

    nodes = {}

    for routeKey, routeMap in requests.get('http://localhost:12345').json().iteritems():
        for i, node in enumerate(routeMap['Path']):
            g.add_node(node['Name'])
            labels['nodes'][node['Name']] = node['Name']
            if i - 1 >= 0:
                g.add_edge(routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])
                labels['edges'][(routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])] = (routeMap['Path'][i-1]['Name'], routeMap['Path'][i]['Name'])


    #pos = nx.spring_layout(g)
    #nx.draw(g, pos=pos)
    nx.draw_networkx(g, with_labels=True)

    # add labels
    #nx.draw_networkx_labels(g, pos, labels['nodes'])
    #nx.draw_networkx_edge_labels(g, pos, labels['edges'])

    # write out the graph
    plt.savefig('dag.png')
    plt.show()  # in case people have the required libraries to make it happen
